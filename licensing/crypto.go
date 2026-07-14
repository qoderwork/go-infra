// Package licensing — crypto.go
//
// # Optional Encryption Layer
//
// The signing scheme (Ed25519) protects license integrity — nobody can
// forge or tamper with a license without the private key. However, the
// signed Envelope stores the license payload as plaintext base64, so
// anyone who reads the .lic file can see all fields (features, capacity,
// subject, etc.).
//
// This file adds an optional AES-256-GCM encryption layer on top of the
// signing layer. The flow is:
//
//	Sign → Envelope → Encrypt → EncryptedEnvelope    (issuer side)
//	EncryptedEnvelope → Decrypt → Envelope → Verify  (consumer side)
//
// GCM (Galois/Counter Mode) provides authenticated encryption: the
// ciphertext is both confidential and tamper-evident. If an attacker
// modifies the encrypted blob, decryption will fail before any signature
// check is attempted.
//
// # Security Model
//
// The AES key must be distributed to the consumer through a separate
// channel from the license file (two-channel delivery). Typical
// strategies for offline deployments:
//
//   - Embed the AES key in the application binary via go:embed, alongside
//     the public key. The attacker needs to both extract the key from the
//     binary AND obtain the encrypted license file.
//   - Ship the AES key on a separate medium (USB, printed QR code) and
//     have the operator enter it at first-run setup.
//
// # Key Size
//
// Only AES-256 (32-byte key) is supported. Use GenerateAESKey to create
// a cryptographically random key, then persist it securely.
//
// # File Format
//
// An EncryptedEnvelope is serialized as JSON:
//
//	{
//	  "version":    1,
//	  "alg":        "ed25519+aes-256-gcm",
//	  "ciphertext": "<base64 of nonce || AES-GCM ciphertext>"
//	}
//
// The "alg" field distinguishes encrypted envelopes from plain ones, so
// a consumer can auto-detect the format. The ciphertext internally is
// nonce (12 bytes) prepended to the GCM output (ciphertext + 16-byte
// authentication tag), then base64-encoded.
package licensing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// AESKeySize is the required AES key length in bytes (256-bit).
// GenerateAESKey produces keys of exactly this size.
const AESKeySize = 32

// EncryptedEnvelope wraps a signed Envelope in AES-256-GCM encryption.
//
// The license content is opaque — only the holder of the AES key can
// decrypt it to reveal the inner Envelope, which can then be verified
// against the Ed25519 public key as usual.
//
// JSON structure:
//
//	{
//	  "version":    <int, copied from inner Envelope for routing>,
//	  "alg":        "ed25519+aes-256-gcm",
//	  "ciphertext": "<base64(nonce_12 || gcm_ciphertext || gcm_tag_16)>"
//	}
type EncryptedEnvelope struct {
	Version    int    `json:"version"`
	Alg        string `json:"alg"`        // always "ed25519+aes-256-gcm"
	Ciphertext string `json:"ciphertext"` // base64 of AES-GCM(nonce + envelope_json)
}

// ErrNotEncrypted is returned when data is expected to be an encrypted
// envelope but is actually a plain (signed-only) envelope.
var ErrNotEncrypted = errors.New("license: not an encrypted envelope")

// EncryptEnvelope encrypts a signed Envelope with AES-256-GCM, producing
// an EncryptedEnvelope whose license content is hidden from inspection.
//
// The key must be exactly AESKeySize (32) bytes. Use GenerateAESKey to
// create a suitable key.
//
// Usage:
//
//	signer := NewSigner(privKey, CurrentVersion)
//	env, _ := signer.Sign(lic)
//	enc, _ := EncryptEnvelope(env, aesKey)
//	SaveEncryptedEnvelope("license.enc", enc)
func EncryptEnvelope(env *Envelope, key []byte) (*EncryptedEnvelope, error) {
	if len(key) != AESKeySize {
		return nil, fmt.Errorf("license: aes key must be %d bytes, got %d", AESKeySize, len(key))
	}
	envJSON, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("license: marshal envelope for encryption: %w", err)
	}
	ciphertext, err := aesGCMSeal(key, envJSON)
	if err != nil {
		return nil, err
	}
	return &EncryptedEnvelope{
		Version:    env.Version,
		Alg:        "ed25519+aes-256-gcm",
		Ciphertext: ciphertext,
	}, nil
}

// DecryptEnvelope decrypts an EncryptedEnvelope back into a signed Envelope.
// The returned Envelope can be passed to Verifier.VerifyEnvelope for
// signature verification.
//
// The key must match the one used during EncryptEnvelope. A wrong key
// causes GCM authentication to fail, returning ErrMalformed.
func DecryptEnvelope(enc *EncryptedEnvelope, key []byte) (*Envelope, error) {
	if len(key) != AESKeySize {
		return nil, fmt.Errorf("license: aes key must be %d bytes, got %d", AESKeySize, len(key))
	}
	if enc.Alg != "ed25519+aes-256-gcm" {
		return nil, fmt.Errorf("%w: unsupported encrypted alg %q", ErrMalformed, enc.Alg)
	}
	plaintext, err := aesGCMOpen(key, enc.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	var env Envelope
	if err := json.Unmarshal(plaintext, &env); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	return &env, nil
}

// ParseEncryptedEnvelope decodes raw JSON bytes into an EncryptedEnvelope.
// It validates that the alg field is "ed25519+aes-256-gcm" and that the
// ciphertext is non-empty. It does NOT attempt decryption.
func ParseEncryptedEnvelope(data []byte) (*EncryptedEnvelope, error) {
	var enc EncryptedEnvelope
	if err := json.Unmarshal(data, &enc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	if enc.Alg != "ed25519+aes-256-gcm" {
		return nil, fmt.Errorf("%w: unsupported encrypted alg %q", ErrMalformed, enc.Alg)
	}
	if enc.Ciphertext == "" {
		return nil, fmt.Errorf("%w: empty ciphertext", ErrMalformed)
	}
	return &enc, nil
}

// LoadEncryptedEnvelope reads an encrypted envelope from a file path.
// The file is expected to contain JSON as written by SaveEncryptedEnvelope.
func LoadEncryptedEnvelope(path string) (*EncryptedEnvelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseEncryptedEnvelope(data)
}

// SaveEncryptedEnvelope writes an encrypted envelope as pretty-printed JSON
// with file permissions 0600 (owner read/write only).
func SaveEncryptedEnvelope(path string, enc *EncryptedEnvelope) error {
	data, err := json.MarshalIndent(enc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// VerifyEncrypted is a one-step convenience method on Verifier: it parses
// the raw bytes as an EncryptedEnvelope, decrypts with the given AES key,
// and verifies the inner signed envelope — returning the License on success.
//
// This is the recommended entry point for consumers that use encrypted
// licenses:
//
//	verifier := NewVerifier(pubKey, CurrentVersion).
//	    WithFingerprint(machine.Fingerprint)
//	lic, err := verifier.VerifyEncrypted(data, aesKey)
func (v *Verifier) VerifyEncrypted(data []byte, key []byte) (*License, error) {
	enc, err := ParseEncryptedEnvelope(data)
	if err != nil {
		return nil, err
	}
	env, err := DecryptEnvelope(enc, key)
	if err != nil {
		return nil, err
	}
	return v.VerifyEnvelope(env)
}

// GenerateAESKey creates a new cryptographically random 256-bit AES key
// suitable for use with EncryptEnvelope and DecryptEnvelope.
//
// Store the generated key securely — it is a symmetric secret and must be
// protected with the same care as a password. Typical storage strategies:
//
//   - Embed in the binary via //go:embed (compiled-in, not on disk)
//   - Store in a secrets manager (Vault, AWS Secrets Manager, etc.)
//   - Write to a file with restrictive permissions (0600) on the target host
func GenerateAESKey() ([]byte, error) {
	key := make([]byte, AESKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("license: generate aes key: %w", err)
	}
	return key, nil
}

// ---------------------------------------------------------------------------
// Internal AES-GCM helpers
//
// aesGCMSeal encrypts plaintext with AES-256-GCM and returns the result as
// a base64 string. The output format is:
//
//	base64( nonce_12 || ciphertext || auth_tag_16 )
//
// The nonce is randomly generated per call (12 bytes from crypto/rand).
// GCM appends the 16-byte authentication tag to the ciphertext, so the
// total output is len(plaintext) + 28 bytes before base64 encoding.

func aesGCMSeal(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// Seal appends the ciphertext+tag to the nonce prefix.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// aesGCMOpen decrypts a base64-encoded GCM ciphertext produced by aesGCMSeal.
// It splits the nonce from the front, then calls GCM Open which verifies
// the authentication tag before returning the plaintext. A wrong key or
// tampered ciphertext causes Open to return an error.

func aesGCMOpen(key []byte, b64Ciphertext string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(b64Ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize() // 12 bytes
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

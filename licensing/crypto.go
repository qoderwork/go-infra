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
const AESKeySize = 32

// EncryptedEnvelope is a signed Envelope wrapped in AES-256-GCM encryption.
// The license content is hidden from casual inspection — only the holder of
// the AES key can decrypt and then verify the signature.
type EncryptedEnvelope struct {
	Version    int    `json:"version"`
	Alg        string `json:"alg"`        // always "ed25519+aes-256-gcm"
	Ciphertext string `json:"ciphertext"` // base64 of AES-GCM(nonce + envelope_json)
}

// ErrNotEncrypted is returned when data is not an encrypted envelope.
var ErrNotEncrypted = errors.New("license: not an encrypted envelope")

// EncryptEnvelope encrypts a signed Envelope with AES-256-GCM. The key must
// be exactly 32 bytes. Returns an EncryptedEnvelope whose Ciphertext is the
// base64-encoded nonce+ciphertext.
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

// DecryptEnvelope decrypts an EncryptedEnvelope back to a signed Envelope.
// The caller can then pass the result to Verifier.VerifyEnvelope.
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

// ParseEncryptedEnvelope parses raw JSON bytes into an EncryptedEnvelope.
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

// LoadEncryptedEnvelope reads an encrypted envelope from a file.
func LoadEncryptedEnvelope(path string) (*EncryptedEnvelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseEncryptedEnvelope(data)
}

// SaveEncryptedEnvelope writes an encrypted envelope as pretty-printed JSON.
func SaveEncryptedEnvelope(path string, enc *EncryptedEnvelope) error {
	data, err := json.MarshalIndent(enc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// VerifyEncrypted is a convenience method: parses and decrypts an encrypted
// envelope, then verifies the inner signed envelope, returning the License.
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

// GenerateAESKey creates a new random 256-bit AES key.
func GenerateAESKey() ([]byte, error) {
	key := make([]byte, AESKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("license: generate aes key: %w", err)
	}
	return key, nil
}

// --- internal AES-GCM helpers ---

func aesGCMSeal(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

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
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

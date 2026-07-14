package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
)

// Signer produces signed envelopes with a private key.
type Signer struct {
	priv    ed25519.PrivateKey
	version int
}

// NewSigner builds a signer. If version is 0, CurrentVersion is used.
func NewSigner(priv ed25519.PrivateKey, version int) *Signer {
	if version == 0 {
		version = CurrentVersion
	}
	return &Signer{priv: priv, version: version}
}

// Sign marshals the license canonically, signs it with Ed25519, and returns
// an envelope carrying both the canonical license bytes and the signature.
func (s *Signer) Sign(lic *License) (*Envelope, error) {
	if lic.Version == 0 {
		lic.Version = s.version
	}
	licBytes, err := lic.CanonicalBytes()
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(s.priv, licBytes)
	env := &Envelope{
		Version:   s.version,
		Alg:       "ed25519",
		License:   base64.StdEncoding.EncodeToString(licBytes),
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
	return env, nil
}

// SignEncrypted signs the license and then encrypts the resulting envelope
// with AES-256-GCM. The aesKey must be exactly 32 bytes. This produces an
// opaque license file whose contents are hidden from casual inspection.
func (s *Signer) SignEncrypted(lic *License, aesKey []byte) (*EncryptedEnvelope, error) {
	env, err := s.Sign(lic)
	if err != nil {
		return nil, err
	}
	return EncryptEnvelope(env, aesKey)
}

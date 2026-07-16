package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

const (
	privatePEMType = "PRIVATE KEY" // PKCS#8
	publicPEMType  = "PUBLIC KEY"  // PKIX
)

// GenerateKey creates a new Ed25519 key pair. The return order (pub, priv)
// matches the Go standard library convention (ed25519.GenerateKey).
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("license: generate key: %w", err)
	}
	return pub, priv, nil
}

// EncodePrivateKeyPEM encodes a private key in PKCS#8 PEM.
func EncodePrivateKeyPEM(priv ed25519.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("license: marshal private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: privatePEMType, Bytes: der}), nil
}

// EncodePublicKeyPEM encodes a public key in PKIX PEM.
func EncodePublicKeyPEM(pub ed25519.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("license: marshal public key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: publicPEMType, Bytes: der}), nil
}

// DecodePrivateKeyPEM parses a PKCS#8 PEM private key.
func DecodePrivateKeyPEM(data []byte) (ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("license: invalid private key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("license: parse private key: %w", err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("license: not an Ed25519 private key")
	}
	return priv, nil
}

// DecodePublicKeyPEM parses a PKIX PEM public key.
func DecodePublicKeyPEM(data []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("license: invalid public key PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("license: parse public key: %w", err)
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("license: not an Ed25519 public key")
	}
	return pub, nil
}

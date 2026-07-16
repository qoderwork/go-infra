// Package randutil provides random string generation utilities.
package randutil

import (
	cryptorand "crypto/rand"
	"math/rand"
	"sync"
)

// Charset constants for common use cases.
const (
	CharsetAlphaLower    = "abcdefghijklmnopqrstuvwxyz"
	CharsetAlphaUpper    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	CharsetAlpha         = CharsetAlphaLower + CharsetAlphaUpper
	CharsetNumeric       = "0123456789"
	CharsetAlphaNumeric  = CharsetAlpha + CharsetNumeric
	CharsetDNS1035       = CharsetAlphaLower + CharsetNumeric
	CharsetAlphaNumLower = CharsetAlphaLower + CharsetNumeric // lowercase alphanumeric for DNS labels
	CharsetHex           = "0123456789abcdef"
	CharsetURLSafe       = CharsetAlphaNumeric + "-_"
)

// Thread-safe random source for math/rand.
var (
	rng   *rand.Rand
	rngMu sync.Mutex
)

func init() {
	// Use a deterministic seed for math/rand (not needed in Go 1.20+, but kept for compatibility).
	// Go 1.20+ automatically seeds the global random source.
	rng = rand.New(rand.NewSource(rand.Int63()))
}

// String returns a random string of length n using characters from charset.
// Uses math/rand (non-cryptographic, fast).
func String(n int, charset string) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	rngMu.Lock()
	defer rngMu.Unlock()
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// CryptoString returns a random string of length n using characters from charset.
// Uses crypto/rand (cryptographically secure, slower).
func CryptoString(n int, charset string) (string, error) {
	if n <= 0 {
		return "", nil
	}
	b := make([]byte, n)
	chars := []byte(charset)
	// Use ReadFull to avoid partial reads.
	if _, err := cryptorand.Read(b); err != nil {
		return "", err
	}
	// Map random bytes to charset.
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b), nil
}

// Alpha returns a random lowercase alphabetic string of length n.
func Alpha(n int) string {
	return String(n, CharsetAlphaLower)
}

// AlphaUpper returns a random uppercase alphabetic string of length n.
func AlphaUpper(n int) string {
	return String(n, CharsetAlphaUpper)
}

// AlphaMixed returns a random mixed-case alphabetic string of length n.
func AlphaMixed(n int) string {
	return String(n, CharsetAlpha)
}

// Numeric returns a random numeric string of length n.
func Numeric(n int) string {
	return String(n, CharsetNumeric)
}

// AlphaNumeric returns a random alphanumeric string of length n.
func AlphaNumeric(n int) string {
	return String(n, CharsetAlphaNumeric)
}

// Hex returns a random lowercase hexadecimal string of length n.
func Hex(n int) string {
	return String(n, CharsetHex)
}

// DNS1035 returns a random string suitable for DNS-1035 labels (lowercase alphanumeric).
// DNS-1035 labels must start with a letter and be lowercase alphanumeric.
// Use DNS1035Label for stricter validation.
func DNS1035(n int) string {
	return String(n, CharsetDNS1035)
}

// DNS1035Label returns a random DNS-1035 compliant label of length n.
// DNS-1035: must start with a letter, contain only lowercase alphanumeric and hyphens,
// and end with an alphanumeric character.
func DNS1035Label(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	rngMu.Lock()
	defer rngMu.Unlock()
	// First character must be a letter.
	b[0] = CharsetAlphaLower[rng.Intn(len(CharsetAlphaLower))]
	// Middle characters: lowercase alphanumeric.
	for i := 1; i < n-1; i++ {
		b[i] = CharsetDNS1035[rng.Intn(len(CharsetDNS1035))]
	}
	// Last character: alphanumeric (no hyphen).
	if n > 1 {
		b[n-1] = CharsetAlphaNumLower[rng.Intn(len(CharsetAlphaNumLower))]
	}
	return string(b)
}

// URLSafe returns a random URL-safe string of length n.
func URLSafe(n int) string {
	return String(n, CharsetURLSafe)
}

// CryptoAlpha returns a cryptographically secure random lowercase alphabetic string.
func CryptoAlpha(n int) (string, error) {
	return CryptoString(n, CharsetAlphaLower)
}

// CryptoAlphaNumeric returns a cryptographically secure random alphanumeric string.
func CryptoAlphaNumeric(n int) (string, error) {
	return CryptoString(n, CharsetAlphaNumeric)
}

// CryptoHex returns a cryptographically secure random hexadecimal string.
func CryptoHex(n int) (string, error) {
	return CryptoString(n, CharsetHex)
}

// Bytes returns n random bytes using crypto/rand.
func Bytes(n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	b := make([]byte, n)
	if _, err := cryptorand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}
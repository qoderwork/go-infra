package randutil

import (
	"strings"
	"testing"

	"github.com/qoderwork/go-infra/assert"
)

func TestStringEmptyCharsetNoPanic(t *testing.T) {
	// Empty charset must yield "" instead of panicking on Intn(0).
	assert.Equal(t, "", String(5, ""))
	assert.Equal(t, "", String(0, CharsetAlpha))
}

func TestCryptoStringEmptyCharsetError(t *testing.T) {
	s, err := CryptoString(5, "")
	assert.Error(t, err, "empty charset must be rejected, not divide-by-zero")
	assert.Equal(t, "", s)
}

func TestCryptoStringMembershipAndLength(t *testing.T) {
	const charset = "abc"
	s, err := CryptoString(2000, charset)
	assert.NoError(t, err)
	assert.Equal(t, 2000, len(s))
	for _, r := range s {
		assert.True(t, strings.ContainsRune(charset, r), "unexpected char %q", r)
	}
}

// TestCryptoStringFullCoverage guards against an index-mapping bug that drops
// tail characters: every character of the charset must appear over many draws.
func TestCryptoStringFullCoverage(t *testing.T) {
	const charset = "abcdefghij" // length 10 (not a power of two)
	s, err := CryptoString(20000, charset)
	assert.NoError(t, err)
	seen := make(map[rune]int)
	for _, r := range s {
		seen[r]++
	}
	for _, c := range charset {
		assert.True(t, seen[c] > 0, "charset char %q never produced", c)
	}
}

package machine

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/qoderwork/go-infra/assert"
)

// uuidV5 shape: 8-4-4(v5)-4(variant 8/9/a/b)-12, lower hex.
var uuidV5Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNormalizeProducesUUIDv5Shape(t *testing.T) {
	got := normalize("board:12345")
	assert.True(t, uuidV5Re.MatchString(got), "normalize output %q is not a UUID v5 shape", got)
	// Deterministic: same input -> same output.
	assert.Equal(t, got, normalize("board:12345"))
	// Surrounding whitespace is trimmed before hashing.
	assert.Equal(t, got, normalize("  board:12345  "))
	// Different input -> different output.
	if normalize("board:12345") == normalize("board:99999") {
		t.Error("different inputs produced the same normalized fingerprint")
	}
}

func TestSystemUUIDToken(t *testing.T) {
	tok := systemUUIDToken("4c4c4544-004e-4d10-8034-b4a44c4c4634")
	assert.Equal(t, "sysuuid:4C4C4544004E4D108034B4A44C4C4634", tok)

	// Placeholder / empty yield "" so callers fall through to the next source.
	assert.Equal(t, "", systemUUIDToken(""))
	assert.Equal(t, "", systemUUIDToken("Not Specified"))
	assert.Equal(t, "", systemUUIDToken("unknown"))
}

func TestFingerprintFromSystemUUIDRoundtrip(t *testing.T) {
	fp, err := FingerprintFromSystemUUID("a1b2c3d4-1111-2222-3333-444455556666")
	assert.NoError(t, err)
	assert.True(t, uuidV5Re.MatchString(fp), "fingerprint %q not UUID v5", fp)

	_, err = FingerprintFromSystemUUID("")
	assert.Error(t, err, "empty system UUID must be rejected")

	_, err = FingerprintFromSystemUUID("unknown")
	assert.Error(t, err, "placeholder system UUID must be rejected")

	// The token equals what Fingerprint would compute for the same host value.
	want := normalize(systemUUIDToken("a1b2c3d4-1111-2222-3333-444455556666"))
	assert.Equal(t, want, fp)
}

func TestReadFileTrim(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "id")
	assert.NoError(t, os.WriteFile(p, []byte("  abc123\n\n"), 0o644))
	assert.Equal(t, "abc123", readFileTrim(p))

	// Missing file reads as empty rather than panicking.
	assert.Equal(t, "", readFileTrim(filepath.Join(dir, "missing")))
}

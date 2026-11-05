package system

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qoderwork/go-infra/assert"
)

func TestGetDirSizeAndGlobAndClean(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a")
	assert.NoError(t, os.MkdirAll(sub, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(sub, "f1.txt"), []byte("12345"), 0o644))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "f2.txt"), []byte("123"), 0o644))

	size, err := GetDirSize(dir)
	assert.NoError(t, err)
	assert.Equal(t, int64(5+3), size, "GetDirSize sums regular file bytes")

	files, err := GlobFiles(dir)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(files), "GlobFiles returns only regular files")

	removed, err := CleanOldFiles(files, 1)
	assert.NoError(t, err)
	assert.Equal(t, 1, removed, "one file removed")
	remaining, err := GlobFiles(dir)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(remaining))
}

func TestRemoveEmptyDirs(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	other := filepath.Join(dir, "other")
	assert.NoError(t, os.MkdirAll(empty, 0o755))
	assert.NoError(t, os.MkdirAll(other, 0o755))
	assert.NoError(t, os.WriteFile(filepath.Join(other, "keep.txt"), []byte("k"), 0o644))

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)
	assert.NoError(t, RemoveEmptyDirs(context.Background(), dir, start, end))

	_, err := os.Stat(empty)
	assert.True(t, os.IsNotExist(err), "empty subdir must be removed")
	_, err = os.Stat(other)
	assert.NoError(t, err, "subdir with a file must be kept")
}

func TestPortUsed(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	assert.NoError(t, err)
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	assert.True(t, PortUsed("tcp", port), "a bound port is reported as used")
	assert.True(t, PortUsed("tcp", 70000), "out-of-range port is reported as used")
}

func TestCompareVersionFunc(t *testing.T) {
	eq := func(a, b string) bool { return a == b }
	lt := func(a, b string) bool { return a < b }
	assert.True(t, CompareVersionFunc("1.2.3", "1.2.3", eq))
	assert.False(t, CompareVersionFunc("1.2.3", "1.2.4", eq))
	// Missing segments are zero-padded, not treated as "unknown/equal".
	assert.True(t, CompareVersionFunc("1.2", "1.2.0", eq))
	assert.False(t, CompareVersionFunc("1.2", "1.2.3", eq))
	// Segments >= 1000 must not change the normalized width.
	assert.False(t, CompareVersionFunc("1.2.10", "1.2.1000", eq))
	assert.True(t, CompareVersionFunc("1.2.3", "1.2.10", lt))
	// A pre-release suffix is ignored, as in the previous implementation.
	assert.True(t, CompareVersionFunc("1.2.3-beta", "1.2.3", eq))
}

func TestExecutableAndGetwd(t *testing.T) {
	assert.NotEmpty(t, Executable())
	assert.NotEmpty(t, Getwd())
}

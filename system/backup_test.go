package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qoderwork/go-infra/assert"
)

// TestFileBackupFlushesOnClose verifies a single accepted update is persisted to
// the source path by Close, even when the background writer had not drained the
// buffer yet, and that no .back residue is left behind.
func TestFileBackupFlushesOnClose(t *testing.T) {
	target := filepath.Join(t.TempDir(), "data.txt")

	fb := NewFileBackup(target)
	assert.True(t, fb.Write([]byte("hello")), "first Write must be accepted")
	fb.Close() // Close flushes the pending update and waits for the goroutine

	data, err := os.ReadFile(target)
	assert.NoError(t, err, "source file must exist after Close")
	assert.Equal(t, "hello", string(data))

	_, err = os.Stat(target + ".back")
	assert.True(t, os.IsNotExist(err), "temporary .back file must be renamed away")
}

// TestFileBackupMultipleWritesConverge verifies repeated updates leave the source
// file readable and equal to the last accepted value once Close returns.
func TestFileBackupMultipleWritesConverge(t *testing.T) {
	target := filepath.Join(t.TempDir(), "state.json")
	fb := NewFileBackup(target)

	last := ""
	for _, v := range []string{"v1", "v2", "v3", "v4"} {
		if fb.Write([]byte(v)) {
			last = v
		}
		// give the writer a chance to drain so subsequent writes are accepted
		time.Sleep(10 * time.Millisecond)
	}
	fb.Close()

	data, err := os.ReadFile(target)
	assert.NoError(t, err)
	assert.Equal(t, last, string(data), "file must hold the last accepted update")
}

// TestFileBackupCloseIdempotent verifies Close can be called repeatedly without
// panicking or blocking (sync.Once guard + done channel).
func TestFileBackupCloseIdempotent(t *testing.T) {
	target := filepath.Join(t.TempDir(), "x.txt")
	fb := NewFileBackup(target)
	assert.True(t, fb.Write([]byte("x")))

	done := make(chan struct{})
	go func() { fb.Close(); fb.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked or repeated Close deadlocked")
	}
}

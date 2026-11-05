package system

import (
	"log/slog"
	"os"
	"sync"
)

// FileBackup asynchronously persists data to sourcePath via a write-then-rename
// sequence on a dedicated background goroutine. The rename makes the update
// atomic against readers of sourcePath on POSIX-like filesystems.
//
// It keeps a single-slot buffer: the most recent accepted update is written,
// and Write reports when a newer update arrives before the previous one has
// been flushed.
type FileBackup struct {
	sourcePath string
	backupPath string

	ch        chan []byte
	quit      chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

// NewFileBackup starts a background writer for file. Close it when done to
// flush any pending update and stop the goroutine.
func NewFileBackup(file string) *FileBackup {
	f := &FileBackup{
		sourcePath: file,
		backupPath: file + ".back",
		ch:         make(chan []byte, 1),
		quit:       make(chan struct{}),
		done:       make(chan struct{}),
	}
	go f.start()
	return f
}

func (f *FileBackup) start() {
	defer close(f.done)
	for {
		select {
		case <-f.quit:
			// Flush the last buffered update (if any) before exiting so a
			// Write that already returned true is not silently lost.
			select {
			case data := <-f.ch:
				f.persist(data)
			default:
			}
			return
		case data := <-f.ch:
			f.persist(data)
		}
	}
}

func (f *FileBackup) persist(data []byte) {
	if err := os.WriteFile(f.backupPath, data, 0o644); err != nil {
		slog.Error("FileBackup write", "path", f.backupPath, "err", err)
		return
	}
	if err := os.Rename(f.backupPath, f.sourcePath); err != nil {
		slog.Error("FileBackup rename", "path", f.sourcePath, "err", err)
	}
}

// Write hands data to the background writer without blocking. It returns true
// if the update was accepted, or false when the buffer is still holding an
// unflushed update — in that case data was NOT written and the caller may retry
// or drop it deliberately. Write must not be called after Close.
func (f *FileBackup) Write(data []byte) bool {
	select {
	case f.ch <- data:
		return true
	default:
		return false
	}
}

// Close signals the writer to stop, flushes one pending update, and blocks
// until the background goroutine has exited. It is safe to call concurrently
// and repeatedly. Callers must stop writing before closing.
func (f *FileBackup) Close() {
	f.closeOnce.Do(func() { close(f.quit) })
	<-f.done
}

package conc

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// countCleanupGoroutines looks for the tickerCleanup frame in all goroutine
// stacks. Unlike runtime.NumGoroutine it is not affected by unrelated
// packages running in parallel during `go test ./...`.
func countCleanupGoroutines() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	// Count stack frames, not incidental mentions in source paths.
	return strings.Count(string(buf[:n]), ".tickerCleanup(")
}

func waitForCleanupGoroutines(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for countCleanupGoroutines() > want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := countCleanupGoroutines(); got > want {
		t.Fatalf("cleanup goroutines leaked: want <= %d, got %d", want, got)
	}
}

func TestTTLCacheCloseReleasesCleanupGoroutine(t *testing.T) {
	base := countCleanupGoroutines()
	caches := make([]*TTLCache, 20)
	for i := range caches {
		caches[i] = NewTTLCache(time.Minute)
	}
	// Goroutines start asynchronously, so do not assert how many have been
	// scheduled at this point. The contract under test is that Close()
	// releases every one of them.
	for _, c := range caches {
		c.Close()
	}
	waitForCleanupGoroutines(t, base)
}

func TestTTLMapDisposeReleasesCleanupGoroutine(t *testing.T) {
	base := countCleanupGoroutines()
	maps := make([]*TTLMap[string, int], 20)
	for i := range maps {
		maps[i] = NewTTLMap[string, int]()
	}
	for _, m := range maps {
		m.Dispose()
	}
	waitForCleanupGoroutines(t, base)
}

// TestTTLMapLoadOrStoreHasExpiryWithValue locks in the write order: Load()
// checks exp first, so storing exp before data used to expose a window where
// a freshly stored key was invisible.
func TestTTLMapLoadOrStoreHasExpiryWithValue(t *testing.T) {
	m := NewTTLMap[string, int]()
	defer m.Dispose()

	v, loaded := m.LoadOrStore("k", 7, time.Minute)
	if loaded || v != 7 {
		t.Fatalf("LoadOrStore = (%d, %v), want (7, false)", v, loaded)
	}
	if got, ok := m.Load("k"); !ok || got != 7 {
		t.Fatalf("Load after LoadOrStore = (%d, %v), want (7, true)", got, ok)
	}
	// Loading an existing key must not refresh its expiry.
	if _, loaded := m.LoadOrStore("k", 99, time.Nanosecond); !loaded {
		t.Fatal("second LoadOrStore should load the existing value")
	}
}

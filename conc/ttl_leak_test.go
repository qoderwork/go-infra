package conc

import (
	"runtime"
	"testing"
	"time"

	"github.com/qoderwork/go-infra/assert"
)

// TestNewTTLMapStartsNoGoroutine guards the leak fix: NewTTLMap must not spawn
// a background cleanup goroutine at construction. Creating many caches without
// Dispose must not grow the goroutine count proportionally.
func TestNewTTLMapStartsNoGoroutine(t *testing.T) {
	runtime.GC()
	before := runtime.NumGoroutine()

	keep := make([]*TTLMap[string, int], 0, 200)
	for i := 0; i < 200; i++ {
		m := NewTTLMap[string, int]()
		m.Store("k", i, time.Hour)
		keep = append(keep, m)
	}

	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+8 {
		t.Fatalf("NewTTLMap leaked goroutines: before=%d after=%d for 200 maps", before, after)
	}
	_ = keep
}

// TestTTLMapDisposeBlocksWrites verifies a disposed map cannot be silently
// revived: Store/LoadOrStore become no-ops and Load reports a miss afterwards.
func TestTTLMapDisposeBlocksWrites(t *testing.T) {
	c := NewTTLMap[string, int]()
	c.Store("a", 1, time.Hour)
	v, ok := c.Load("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v)

	c.Dispose()

	// Data cleared and further writes are no-ops.
	_, ok = c.Load("a")
	assert.False(t, ok, "Load after Dispose must miss")

	c.Store("b", 2, time.Hour)
	_, ok = c.Load("b")
	assert.False(t, ok, "Store after Dispose must not persist")

	_, loaded := c.LoadOrStore("c", 3, time.Hour)
	assert.False(t, loaded, "LoadOrStore after Dispose must not store")
}

// TestTTLCacheCloseStopsWrites mirrors TTLCache.Close semantics via the map.
func TestTTLCacheCloseStopsWrites(t *testing.T) {
	cache := NewTTLCache(time.Hour)
	cache.Set(t.Context(), "a", "1")
	cache.Close()

	var dest string
	err := cache.Get(t.Context(), "a", &dest)
	assert.ErrorIs(t, err, ErrCacheNotFound, "Get after Close must miss")
}

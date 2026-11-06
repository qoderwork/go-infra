package cache

import (
	"sync"
	"testing"

	"github.com/qoderwork/go-infra/assert"
)

func TestLRURemoveOldest(t *testing.T) {
	c := NewLRU(2)
	c.Add("a", 1)
	c.Add("b", 2)
	c.RemoveOldest() // "a" is the least recently used

	_, ok := c.Get("a")
	assert.False(t, ok, "a should be removed")
	if v, ok := c.Get("b"); !ok || v.(int) != 2 {
		t.Fatalf("b = %v, %v, want 2 true", v, ok)
	}
	assert.Equal(t, 1, c.Len())
}

func TestLRUEvictedCallbackReceivesKeyAndValue(t *testing.T) {
	type ev struct {
		k Key
		v int
	}
	var got []ev
	c := NewLRU(2)
	c.OnEvicted = func(k Key, v any) { got = append(got, ev{k, v.(int)}) }

	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3) // evicts "a"

	assert.Len(t, got, 1)
	assert.Equal(t, Key("a"), got[0].k)
	assert.Equal(t, 1, got[0].v)
}

func TestLRUKeysMostRecentlyUsedFirst(t *testing.T) {
	c := NewLRU(5)
	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3)
	c.Get("a") // touch "a" so it becomes most recent

	keys := c.Keys()
	assert.Len(t, keys, 3)
	assert.Equal(t, Key("a"), keys[0], "most recently used must come first")
}

// TestLRUPeekDoesNotReorder verifies Peek returns the value without changing
// recency, whereas Get does.
func TestLRUPeekDoesNotReorder(t *testing.T) {
	c := NewLRU(5)
	c.Add("a", 1)
	c.Add("b", 2)

	v, ok := c.Peek("a")
	assert.True(t, ok)
	assert.Equal(t, 1, v.(int))
	// Recency unchanged: "a" was added first and only Peeked, so "b" stays newest.
	assert.Equal(t, Key("b"), c.Keys()[0], "Peek must not move the key to front")

	c.Get("a") // now it becomes most recently used
	assert.Equal(t, Key("a"), c.Keys()[0])
}

func TestThreadSafeLRUGetMissReturnsZeroValue(t *testing.T) {
	c := NewThreadSafeLRU(3)
	v, ok := c.Get("missing")
	assert.False(t, ok)
	assert.Nil(t, v, "missing key must yield a nil interface value")
}

func TestThreadSafeLRUPeekConcurrent(t *testing.T) {
	c := NewThreadSafeLRU(64)
	for i := 0; i < 64; i++ {
		c.Add(i, i)
	}
	// Peek takes only a read lock: many concurrent Peeks must not race or block.
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 64; i++ {
				v, ok := c.Peek(i)
				if !ok || v.(int) != i {
					t.Errorf("Peek(%d) = %v, %v", i, v, ok)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestThreadSafeLRURemoveOldestAndClear(t *testing.T) {
	c := NewThreadSafeLRU(2)
	c.Add("a", 1)
	c.Add("b", 2)
	c.RemoveOldest()
	_, ok := c.Get("a")
	assert.False(t, ok, "oldest removed")

	c.Add("c", 3)
	c.Clear()
	assert.Equal(t, 0, c.Len())
}

// TestLRUPeekDoesNotChangeRecency is the reason Peek exists: looking up a key
// must not promote it, so it stays the eviction candidate.
func TestLRUPeekDoesNotChangeRecency(t *testing.T) {
	c := NewLRU(2)
	c.Add("a", 1)
	c.Add("b", 2)
	if v, ok := c.Peek("a"); !ok || v.(int) != 1 {
		t.Fatalf("Peek(a) = %v, %v, want 1 true", v, ok)
	}
	c.Add("c", 3) // evicts "a" only if Peek left it least recently used
	if _, ok := c.Get("a"); ok {
		t.Fatal("Peek must not mark a as recently used")
	}
	if v, ok := c.Get("c"); !ok || v.(int) != 3 {
		t.Fatalf("c = %v, %v, want 3 true", v, ok)
	}
}

func TestThreadSafeLRUPeek(t *testing.T) {
	c := NewThreadSafeLRU(2)
	c.Add("a", 1)
	if v, ok := c.Peek("a"); !ok || v.(int) != 1 {
		t.Fatalf("Peek(a) = %v, %v, want 1 true", v, ok)
	}
	if v, ok := c.Peek("missing"); ok || v != nil {
		t.Fatalf("Peek(missing) = %v, %v, want nil false", v, ok)
	}
}

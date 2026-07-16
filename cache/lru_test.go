package cache

import (
	"sync"
	"testing"
)

func TestLRUBasic(t *testing.T) {
	c := NewLRU(3)
	if c.Len() != 0 {
		t.Fatalf("initial len = %d, want 0", c.Len())
	}

	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3)

	if c.Len() != 3 {
		t.Fatalf("len after 3 adds = %d, want 3", c.Len())
	}

	// Get existing key.
	v, ok := c.Get("a")
	if !ok || v.(int) != 1 {
		t.Fatalf("get a = %v, %v, want 1, true", v, ok)
	}

	// Add 4th item, should evict "b" (oldest).
	c.Add("d", 4)
	if c.Len() != 3 {
		t.Fatalf("len after 4th add = %d, want 3", c.Len())
	}

	_, ok = c.Get("b")
	if ok {
		t.Fatal("b should have been evicted")
	}

	// Verify remaining keys.
	for _, k := range []Key{"a", "c", "d"} {
		v, ok := c.Get(k)
		if !ok {
			t.Fatalf("key %v should exist", k)
		}
		_ = v
	}
}

func TestLRURemove(t *testing.T) {
	c := NewLRU(10)
	c.Add("a", 1)
	c.Remove("a")
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should have been removed")
	}
}

func TestLRUClear(t *testing.T) {
	var evicted []Key
	c := NewLRU(10)
	c.OnEvicted = func(k Key, v interface{}) {
		evicted = append(evicted, k)
	}
	c.Add("a", 1)
	c.Add("b", 2)
	c.Clear()
	if c.Len() != 0 {
		t.Fatalf("len after clear = %d, want 0", c.Len())
	}
	if len(evicted) != 2 {
		t.Fatalf("evicted count = %d, want 2", len(evicted))
	}
}

func TestLRUKeys(t *testing.T) {
	c := NewLRU(10)
	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3)
	// Get "a" to move it to front.
	c.Get("a")

	keys := c.Keys()
	// Order: most recently used first. "a" was accessed last.
	if len(keys) != 3 {
		t.Fatalf("len(keys) = %d, want 3", len(keys))
	}
	// Expected order: a, c, b (a accessed last, then c added, then b)
	if keys[0] != Key("a") {
		t.Fatalf("keys[0] = %v, want a", keys[0])
	}
}

func TestThreadSafeLRUConcurrent(t *testing.T) {
	c := NewThreadSafeLRU(100)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Add(id*100+j, j)
			}
		}(i)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				c.Get(id*100 + j)
			}
		}(i)
	}
	wg.Wait()
	if c.Len() > 100 {
		t.Fatalf("len = %d, should be <= 100", c.Len())
	}
}

func TestThreadSafeLRUEvictionCallback(t *testing.T) {
	var mu sync.Mutex
	evicted := make(map[Key]int)
	c := NewThreadSafeLRUWithEviction(2, func(k Key, v interface{}) {
		mu.Lock()
		evicted[k] = v.(int)
		mu.Unlock()
	})

	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3) // should evict "a"

	mu.Lock()
	if evicted["a"] != 1 {
		t.Fatalf("evicted[a] = %d, want 1", evicted["a"])
	}
	mu.Unlock()
}

func TestLRUZeroMaxEntries(t *testing.T) {
	c := NewLRU(0)
	c.Add("a", 1)
	c.Add("b", 2)
	c.Add("c", 3)
	// No eviction should happen.
	if c.Len() != 3 {
		t.Fatalf("len = %d, want 3 (no limit)", c.Len())
	}
}
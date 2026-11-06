package cache

import "sync"

// ThreadSafeLRU is a thread-safe LRU cache.
type ThreadSafeLRU struct {
	cache     *LRU
	mu        sync.RWMutex
	onEvicted func(key Key, value any)
	// evicted collects entries evicted by the inner LRU while the lock is held.
	// The real callback is fired after the lock is released (see fireEvicted).
	evicted []entry
}

// NewThreadSafeLRU creates a new thread-safe LRU cache of the given size.
func NewThreadSafeLRU(maxEntries int) *ThreadSafeLRU {
	return &ThreadSafeLRU{
		cache: NewLRU(maxEntries),
	}
}

// NewThreadSafeLRUWithEviction creates a new thread-safe LRU cache with an eviction callback.
// The callback is invoked outside the lock, so it is safe to call back into the cache.
func NewThreadSafeLRUWithEviction(maxEntries int, onEvicted func(key Key, value any)) *ThreadSafeLRU {
	c := NewThreadSafeLRU(maxEntries)
	c.onEvicted = onEvicted
	c.cache.OnEvicted = c.collectEvicted
	return c
}

// collectEvicted is installed as the inner LRU's OnEvicted callback. It runs while
// c.mu is held (during the inner LRU's operations), so it only records the pending
// evictions; the real callback is invoked after the lock is released to avoid a
// deadlock when the callback re-enters the cache.
func (c *ThreadSafeLRU) collectEvicted(key Key, value any) {
	c.evicted = append(c.evicted, entry{key: key, value: value})
}

// takeEvicted returns and clears the pending evictions. It must be called while
// holding c.mu so concurrent operations do not race on the slice.
func (c *ThreadSafeLRU) takeEvicted() []entry {
	evicted := c.evicted
	c.evicted = nil
	return evicted
}

// fireEvicted invokes the user callback for each evicted entry. It must be called
// outside c.mu.
func (c *ThreadSafeLRU) fireEvicted(evicted []entry) {
	if c.onEvicted == nil {
		return
	}
	for _, e := range evicted {
		c.onEvicted(e.key, e.value)
	}
}

// Add adds a value to the cache.
func (c *ThreadSafeLRU) Add(key Key, value any) {
	c.mu.Lock()
	c.cache.Add(key, value)
	evicted := c.takeEvicted()
	c.mu.Unlock()
	c.fireEvicted(evicted)
}

// Get looks up a key's value and marks it as most recently used. It must take
// the write lock because reordering mutates the list. Use Peek for a lookup
// that does not touch recency and can run under a read lock.
func (c *ThreadSafeLRU) Get(key Key) (value any, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.Get(key)
}

// Peek returns a key's value without updating its recency order. Because it
// mutates nothing, it takes only a read lock, so concurrent Peeks scale where
// concurrent Gets serialize on the write lock.
func (c *ThreadSafeLRU) Peek(key Key) (value any, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Peek(key)
}

// Remove removes the provided key from the cache.
func (c *ThreadSafeLRU) Remove(key Key) {
	c.mu.Lock()
	c.cache.Remove(key)
	evicted := c.takeEvicted()
	c.mu.Unlock()
	c.fireEvicted(evicted)
}

// RemoveOldest removes the oldest item from the cache.
func (c *ThreadSafeLRU) RemoveOldest() {
	c.mu.Lock()
	c.cache.RemoveOldest()
	evicted := c.takeEvicted()
	c.mu.Unlock()
	c.fireEvicted(evicted)
}

// Len returns the number of items in the cache.
func (c *ThreadSafeLRU) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Len()
}

// Clear purges all stored items from the cache.
func (c *ThreadSafeLRU) Clear() {
	c.mu.Lock()
	c.cache.Clear()
	evicted := c.takeEvicted()
	c.mu.Unlock()
	c.fireEvicted(evicted)
}

// Keys returns all keys in the cache (from most recently used to least).
func (c *ThreadSafeLRU) Keys() []Key {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Keys()
}

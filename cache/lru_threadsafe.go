package cache

import "sync"

// ThreadSafeLRU is a thread-safe LRU cache.
type ThreadSafeLRU struct {
	cache *LRU
	mu    sync.RWMutex
}

// NewThreadSafeLRU creates a new thread-safe LRU cache of the given size.
func NewThreadSafeLRU(maxEntries int) *ThreadSafeLRU {
	return &ThreadSafeLRU{
		cache: NewLRU(maxEntries),
	}
}

// NewThreadSafeLRUWithEviction creates a new thread-safe LRU cache with an eviction callback.
func NewThreadSafeLRUWithEviction(maxEntries int, onEvicted func(key Key, value interface{})) *ThreadSafeLRU {
	c := NewThreadSafeLRU(maxEntries)
	c.cache.OnEvicted = onEvicted
	return c
}

// Add adds a value to the cache.
func (c *ThreadSafeLRU) Add(key Key, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Add(key, value)
}

// Get looks up a key's value from the cache.
func (c *ThreadSafeLRU) Get(key Key) (value interface{}, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.Get(key)
}

// Remove removes the provided key from the cache.
func (c *ThreadSafeLRU) Remove(key Key) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Remove(key)
}

// RemoveOldest removes the oldest item from the cache.
func (c *ThreadSafeLRU) RemoveOldest() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.RemoveOldest()
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
	defer c.mu.Unlock()
	c.cache.Clear()
}

// Keys returns all keys in the cache (from most recently used to least).
func (c *ThreadSafeLRU) Keys() []Key {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Keys()
}
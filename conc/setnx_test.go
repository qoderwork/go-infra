package conc

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/qoderwork/go-infra/assert"
)

// TestSetNXPrefilledNoOverwrite verifies SetNX leaves an existing live key
// untouched.
func TestSetNXPrefilledNoOverwrite(t *testing.T) {
	c := NewTTLCache(time.Hour)
	defer c.Close()
	c.Set(t.Context(), "k", "init")

	c.SetNX(t.Context(), "k", "other")
	var got string
	assert.NoError(t, c.Get(t.Context(), "k", &got))
	assert.Equal(t, "init", got, "SetNX must not overwrite an existing key")
}

// TestSetNXConcurrent exercises the LoadOrStore-based atomic guard: many
// goroutines race to populate the same fresh keys. Every key must end up
// readable. Run under -race this also checks for unsynchronized map access
// (the old Load+Store pair could race). Non-overwrite semantics are covered
// separately by TestSetNXPrefilledNoOverwrite.
func TestSetNXConcurrentSingleWinner(t *testing.T) {
	c := NewTTLCache(time.Hour)
	defer c.Close()

	const keys = 50
	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < keys; i++ {
				c.SetNX(t.Context(), strconv.Itoa(i), id)
			}
		}(g)
	}
	wg.Wait()

	for i := 0; i < keys; i++ {
		var v int
		if err := c.Get(t.Context(), strconv.Itoa(i), &v); err != nil {
			t.Fatalf("key %d expected to be present: %v", i, err)
		}
	}
}

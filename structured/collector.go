// Copyright (c) 2025 Umputun
// Derived from github.com/go-pkgz/pool (MIT License), file: collector.go.
// Modified by qoderwork: Close is idempotent, package renamed to structured.
// See THIRD_PARTY_LICENSES.md for the full license text.
package structured

import (
	"context"
	"iter"
	"sync"
)

// Collector bridges asynchronous producers and a synchronous consumer.
//
// Workers Submit values, then the owner closes the collector and ranges over
// Iter (or calls All). Submit must not be called after Close, and all
// outstanding Submit calls must finish before Close.
type Collector[V any] struct {
	ch        chan V
	ctx       context.Context
	closeOnce sync.Once
}

// NewCollector creates a collector with a buffered channel of the given size.
// The context is used to abort iteration; it does not unblock a Submit that is
// waiting on a full channel, so use a context that is also respected by the
// producers.
func NewCollector[V any](ctx context.Context, size int) *Collector[V] {
	if ctx == nil {
		ctx = context.Background()
	}
	if size < 0 {
		size = 0
	}
	return &Collector[V]{ch: make(chan V, size), ctx: ctx}
}

// Submit sends a value to the collector. It blocks while the buffer is full
// (the context does NOT unblock it) and panics if the collector is closed.
// Size the buffer for all outstanding submissions, or start All/Iter in a
// separate goroutine before submitting.
func (c *Collector[V]) Submit(v V) {
	c.ch <- v
}

// Close closes the collector. Repeated calls are safe.
func (c *Collector[V]) Close() {
	c.closeOnce.Do(func() { close(c.ch) })
}

// Iter returns a range-able iterator over the collected values. The second
// value is non-nil when the context is canceled, even if the channel was
// already closed and drained.
func (c *Collector[V]) Iter() iter.Seq2[V, error] {
	return func(yield func(V, error) bool) {
		for {
			select {
			case v, ok := <-c.ch:
				if !ok {
					// A closed channel is always ready, so without this check
					// select could pick it over ctx.Done() and silently turn a
					// canceled collector into a clean end-of-stream.
					if err := c.ctx.Err(); err != nil {
						var zero V
						yield(zero, err)
					}
					return
				}
				if !yield(v, nil) {
					return
				}
			case <-c.ctx.Done():
				var zero V
				yield(zero, c.ctx.Err())
				return
			}
		}
	}
}

// All collects every remaining value, stopping early on a context error.
func (c *Collector[V]) All() (res []V, err error) {
	for v, err := range c.Iter() {
		if err != nil {
			return res, err
		}
		res = append(res, v)
	}
	return res, nil
}

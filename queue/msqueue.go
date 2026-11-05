// Copyright (c) 2021 The Gnet Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Modified by qoderwork: rewritten as a generic, type-safe queue using
// atomic.Pointer instead of unsafe.Pointer and gnet's *Task type.
// Algorithm: Maged M. Michael and M. L. Scott, 1996 (non-blocking FIFO).
// See THIRD_PARTY_LICENSES.md for the Apache-2.0 license text.

package queue

import "sync/atomic"

type msNode[T any] struct {
	value T
	next  atomic.Pointer[msNode[T]]
}

// MSQueue is an unbounded, lock-free, multi-producer/multi-consumer FIFO
// queue based on the Michael-Scott algorithm. The zero value is NOT usable;
// create one with NewMSQueue.
//
// Unlike LockFree, MSQueue never reports "full": it grows as needed and both
// Enqueue and Dequeue are non-blocking (they retry on contention).
//
// Go's garbage collector makes the classic ABA problem impossible here, so
// atomic.Pointer is sufficient without tagged pointers.
type MSQueue[T any] struct {
	head   atomic.Pointer[msNode[T]]
	tail   atomic.Pointer[msNode[T]]
	length atomic.Int64
}

// NewMSQueue creates an empty queue. A sentinel node is installed as both
// head and tail; it never holds a user value.
func NewMSQueue[T any]() *MSQueue[T] {
	q := &MSQueue[T]{}
	n := &msNode[T]{}
	q.head.Store(n)
	q.tail.Store(n)
	return q
}

// Enqueue appends v to the tail. It is safe to call from multiple goroutines.
func (q *MSQueue[T]) Enqueue(v T) {
	n := &msNode[T]{value: v}
	for {
		tail := q.tail.Load()
		next := tail.next.Load()
		if tail != q.tail.Load() {
			continue // tail changed under us, retry
		}
		if next == nil {
			// Tail really is the last node: try to link the new node.
			if tail.next.CompareAndSwap(nil, n) {
				// Link succeeded; help advance tail (best effort).
				q.tail.CompareAndSwap(tail, n)
				q.length.Add(1)
				return
			}
		} else {
			// Tail is falling behind; help it catch up before retrying.
			q.tail.CompareAndSwap(tail, next)
		}
	}
}

// Dequeue removes and returns the head value. The second result is false when
// the queue is empty. It is safe to call from multiple goroutines.
//
// Len may temporarily disagree with Dequeue results while producers and
// consumers are running: the counter is maintained per successful operation,
// not as part of the linearization point.
func (q *MSQueue[T]) Dequeue() (T, bool) {
	for {
		head := q.head.Load()
		tail := q.tail.Load()
		next := head.next.Load()
		if head != q.head.Load() {
			continue // head changed under us, retry
		}
		if head == tail {
			if next == nil {
				var zero T
				return zero, false // genuinely empty
			}
			// Tail is behind; advance it before retrying.
			q.tail.CompareAndSwap(tail, next)
		} else {
			// Read the value BEFORE swinging head: another consumer may
			// otherwise retire this node first.
			v := next.value
			if q.head.CompareAndSwap(head, next) {
				q.length.Add(-1)
				return v, true
			}
		}
	}
}

// IsEmpty reports whether the queue currently has no elements.
// The result is a snapshot and may be stale immediately after it returns.
func (q *MSQueue[T]) IsEmpty() bool { return q.length.Load() == 0 }

// Len returns the approximate number of queued elements.
func (q *MSQueue[T]) Len() int64 { return q.length.Load() }

// Package queue provides queue implementations.
package queue

import (
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

type casSlot struct {
	putNo uint64
	getNo uint64
	value interface{}
}

// LockFree is a lock-free bounded queue based on CAS operations.
// It supports single-producer single-consumer (SPSC) and multiple-producer
// multiple-consumer (MPMC) scenarios.
//
// The queue has a fixed capacity that is rounded up to the nearest power of two.
// Put operations block when the queue is full; Get operations block when empty.
type LockFree struct {
	sleepTime time.Duration
	capacity  uint64
	capMod    uint64
	putPos    uint64
	getPos    uint64
	slots     []casSlot
}

// NewLockFree creates a new lock-free queue with the given capacity.
// Capacity is rounded up to the nearest power of two.
// sleepTime specifies how long to wait when queue is full/empty before retrying.
func NewLockFree(capacity uint64, sleepTime time.Duration) *LockFree {
	capacity = roundToPowerOfTwo(capacity)
	q := &LockFree{
		capacity:  capacity,
		capMod:    capacity - 1,
		sleepTime: sleepTime,
		slots:     make([]casSlot, capacity),
	}
	for i := range q.slots {
		q.slots[i].getNo = uint64(i)
		q.slots[i].putNo = uint64(i)
	}
	// Initialize first slot.
	q.slots[0].getNo = capacity
	q.slots[0].putNo = capacity
	return q
}

// Capacity returns the queue capacity (rounded to power of two).
func (q *LockFree) Capacity() uint64 {
	return q.capacity
}

// String returns a string representation of the queue state.
func (q *LockFree) String() string {
	return fmt.Sprintf("LockFreeQueue{capacity: %d, putPos: %d, getPos: %d}",
		q.capacity, atomic.LoadUint64(&q.putPos), atomic.LoadUint64(&q.getPos))
}

// Len returns the approximate number of items in the queue.
func (q *LockFree) Len() uint64 {
	putPos := atomic.LoadUint64(&q.putPos)
	getPos := atomic.LoadUint64(&q.getPos)
	if putPos > getPos {
		return putPos - getPos
	}
	return 0
}

// Put adds a value to the queue. Returns false if the queue is full.
// This method is safe for concurrent use by multiple producers.
func (q *LockFree) Put(val interface{}) bool {
	putPos := atomic.LoadUint64(&q.putPos)
	getPos := atomic.LoadUint64(&q.getPos)

	// Queue full?
	if putPos-getPos >= q.capacity {
		if q.sleepTime > 0 {
			time.Sleep(q.sleepTime)
		}
		return false
	}

	putPosNew := putPos + 1
	if !atomic.CompareAndSwapUint64(&q.putPos, putPos, putPosNew) {
		runtime.Gosched()
		return false
	}

	slot := &q.slots[putPosNew&q.capMod]
	for {
		getNo := atomic.LoadUint64(&slot.getNo)
		putNo := atomic.LoadUint64(&slot.putNo)
		if putPosNew == putNo && getNo == putNo {
			slot.value = val
			atomic.AddUint64(&slot.putNo, q.capacity)
			return true
		}
		runtime.Gosched()
	}
}

// Get removes and returns a value from the queue.
// Returns nil, false if the queue is empty.
// This method is safe for concurrent use by multiple consumers.
func (q *LockFree) Get() (interface{}, bool) {
	putPos := atomic.LoadUint64(&q.putPos)
	getPos := atomic.LoadUint64(&q.getPos)

	// Queue empty?
	if putPos == getPos {
		if q.sleepTime > 0 {
			time.Sleep(q.sleepTime)
		}
		return nil, false
	}

	getPosNew := getPos + 1
	if !atomic.CompareAndSwapUint64(&q.getPos, getPos, getPosNew) {
		runtime.Gosched()
		return nil, false
	}

	slot := &q.slots[getPosNew&q.capMod]
	for {
		getNo := atomic.LoadUint64(&slot.getNo)
		putNo := atomic.LoadUint64(&slot.putNo)
		if getPosNew == getNo && getNo == putNo-q.capacity {
			val := slot.value
			slot.value = nil
			atomic.AddUint64(&slot.getNo, q.capacity)
			return val, true
		}
		runtime.Gosched()
	}
}

// PutAll adds multiple values to the queue.
// Returns the number of values successfully added.
func (q *LockFree) PutAll(values []interface{}) int {
	putPos := atomic.LoadUint64(&q.putPos)
	getPos := atomic.LoadUint64(&q.getPos)

	available := q.capacity - (putPos - getPos)
	if available == 0 {
		if q.sleepTime > 0 {
			time.Sleep(q.sleepTime)
		}
		return 0
	}

	count := uint64(len(values))
	if count > available {
		count = available
	}

	putPosNew := putPos + count
	if !atomic.CompareAndSwapUint64(&q.putPos, putPos, putPosNew) {
		runtime.Gosched()
		return 0
	}

	for i := uint64(0); i < count; i++ {
		pos := putPos + 1 + i
		slot := &q.slots[pos&q.capMod]
		for {
			getNo := atomic.LoadUint64(&slot.getNo)
			putNo := atomic.LoadUint64(&slot.putNo)
			if pos == putNo && getNo == putNo {
				slot.value = values[i]
				atomic.AddUint64(&slot.putNo, q.capacity)
				break
			}
			runtime.Gosched()
		}
	}
	return int(count)
}

// GetAll removes and returns all values from the queue.
// Returns the values and the count.
func (q *LockFree) GetAll() []interface{} {
	putPos := atomic.LoadUint64(&q.putPos)
	getPos := atomic.LoadUint64(&q.getPos)

	count := putPos - getPos
	if count == 0 {
		return nil
	}

	getPosNew := getPos + count
	if !atomic.CompareAndSwapUint64(&q.getPos, getPos, getPosNew) {
		runtime.Gosched()
		return nil
	}

	result := make([]interface{}, 0, count)
	for i := uint64(0); i < count; i++ {
		pos := getPos + 1 + i
		slot := &q.slots[pos&q.capMod]
		for {
			getNo := atomic.LoadUint64(&slot.getNo)
			putNo := atomic.LoadUint64(&slot.putNo)
			if pos == getNo && getNo == putNo-q.capacity {
				result = append(result, slot.value)
				slot.value = nil
				atomic.AddUint64(&slot.getNo, q.capacity)
				break
			}
			runtime.Gosched()
		}
	}
	return result
}

// roundToPowerOfTwo rounds v up to the nearest power of two.
func roundToPowerOfTwo(v uint64) uint64 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v |= v >> 32
	v++
	return v
}
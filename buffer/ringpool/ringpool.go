// Package ringpool implements a GC-friendly pool of ring buffers.
//
// Derived from gnet/pkg/pool/ringbuffer (https://github.com/panjf2000/gnet),
// licensed under Apache-2.0.
//
// The pool automatically calibrates itself based on usage patterns,
// adjusting the default and maximum buffer sizes to minimize memory waste.
package ringpool

import (
	"math/bits"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/qoderwork/go-infra/buffer/ring"
)

const (
	minBitSize = 6 // 2**6=64 is a CPU cache line size
	steps      = 20

	minSize = 1 << minBitSize

	calibrateCallsThreshold = 42000
	maxPercentile           = 0.95
)

// RingBuffer is the alias of ring.Buffer.
type RingBuffer = ring.Buffer

// Pool represents a ring-buffer pool.
//
// Distinct pools may be used for distinct types of byte buffers.
// Properly determined byte buffer types with their own pools may help to reduce
// memory waste.
type Pool struct {
	calls       [steps]uint64
	calibrating uint64

	defaultSize uint64
	maxSize     uint64

	pool sync.Pool
}

var builtinPool Pool

// Get returns an empty ring buffer from the default pool.
//
// The returned buffer may be returned to the pool via Put.
// This reduces the number of memory allocations required for buffer management.
func Get() *RingBuffer { return builtinPool.Get() }

// Get returns a new ring buffer with zero length from the pool.
//
// The buffer may be returned to the pool via Put after use
// in order to minimize GC overhead.
func (p *Pool) Get() *RingBuffer {
	v := p.pool.Get()
	if v != nil {
		return v.(*RingBuffer)
	}
	return ring.New(int(atomic.LoadUint64(&p.defaultSize)))
}

// Put returns a ring buffer to the default pool.
//
// The buffer must not be touched after returning it to the pool,
// otherwise data races will occur.
func Put(b *RingBuffer) { builtinPool.Put(b) }

// Put releases a ring buffer obtained via Get back to the pool.
//
// The buffer must not be accessed after returning to the pool.
func (p *Pool) Put(b *RingBuffer) {
	idx := index(b.Len())

	if atomic.AddUint64(&p.calls[idx], 1) > calibrateCallsThreshold {
		p.calibrate()
	}

	maxSize := int(atomic.LoadUint64(&p.maxSize))
	if maxSize == 0 || b.Cap() <= maxSize {
		b.Reset()
		p.pool.Put(b)
	}
}

func (p *Pool) calibrate() {
	if !atomic.CompareAndSwapUint64(&p.calibrating, 0, 1) {
		return
	}

	a := make(callSizes, 0, steps)
	var callsSum uint64
	for i := uint64(0); i < steps; i++ {
		calls := atomic.SwapUint64(&p.calls[i], 0)
		callsSum += calls
		a = append(a, callSize{
			calls: calls,
			size:  minSize << i,
		})
	}
	sort.Sort(a)

	defaultSize := a[0].size
	maxSize := defaultSize

	maxSum := uint64(float64(callsSum) * maxPercentile)
	callsSum = 0
	for i := 0; i < steps; i++ {
		if callsSum > maxSum {
			break
		}
		callsSum += a[i].calls
		size := a[i].size
		if size > maxSize {
			maxSize = size
		}
	}

	atomic.StoreUint64(&p.defaultSize, defaultSize)
	atomic.StoreUint64(&p.maxSize, maxSize)

	atomic.StoreUint64(&p.calibrating, 0)
}

type callSize struct {
	calls uint64
	size  uint64
}

type callSizes []callSize

func (ci callSizes) Len() int {
	return len(ci)
}

func (ci callSizes) Less(i, j int) bool {
	return ci[i].calls > ci[j].calls
}

func (ci callSizes) Swap(i, j int) {
	ci[i], ci[j] = ci[j], ci[i]
}

func index(n int) int {
	n--
	n >>= minBitSize
	idx := 0
	if n > 0 {
		idx = bits.Len(uint(n))
	}
	if idx >= steps {
		idx = steps - 1
	}
	return idx
}

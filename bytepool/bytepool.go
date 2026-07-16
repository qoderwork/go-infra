// Package bytepool provides a pool of byte slices consisting of sync.Pool's
// that collect byte slices with different length sizes in powers of two.
//
// Derived from gnet/pkg/pool/byteslice (https://github.com/panjf2000/gnet),
// licensed under Apache 2.0.
package bytepool

import (
	"math"
	"math/bits"
	"sync"
	"unsafe"
)

var builtinPool Pool

// Pool consists of 32 sync.Pool instances, representing byte slices of length
// from 0 to 31 in powers of two (2^0 to 2^31).
type Pool struct {
	pools [32]sync.Pool
}

// Get returns a byte slice with the given length from the built-in pool.
func Get(size int) []byte {
	return builtinPool.Get(size)
}

// Put returns the byte slice to the built-in pool.
func Put(buf []byte) {
	builtinPool.Put(buf)
}

// Get retrieves a byte slice of the requested length from the pool,
// or allocates a new one if none is available.
//
// The returned slice has length size and capacity rounded up to the nearest
// power of two.
func (p *Pool) Get(size int) []byte {
	if size <= 0 {
		return nil
	}
	if size > math.MaxInt32 {
		return make([]byte, size)
	}
	idx := index(uint32(size))
	ptr, _ := p.pools[idx].Get().(*byte)
	if ptr == nil {
		return make([]byte, size, 1<<idx)
	}
	return unsafe.Slice(ptr, 1<<idx)[:size]
}

// Put returns the byte slice to the pool.
//
// The slice must not be used after being returned to the pool.
func (p *Pool) Put(buf []byte) {
	size := cap(buf)
	if size == 0 || size > math.MaxInt32 {
		return
	}
	idx := index(uint32(size))
	if size != 1<<idx {
		idx--
	}
	p.pools[idx].Put(unsafe.SliceData(buf))
}

func index(n uint32) uint32 {
	return uint32(bits.Len32(n - 1))
}
// Package elastic implements an elastic ring-buffer that lazily
// acquires buffers from a pool and returns them when empty.
//
// Derived from gnet/pkg/buffer/elastic (https://github.com/panjf2000/gnet),
// licensed under Apache-2.0.
//
// The elastic ring buffer wraps a ring.Buffer with automatic pool management:
//   - A buffer is lazily obtained from the pool on first write.
//   - The buffer is automatically returned to the pool when it becomes empty.
//   - Done() explicitly returns the buffer to the pool.
package elastic

import (
	"io"

	"github.com/qoderwork/go-infra/buffer/ring"
	rbPool "github.com/qoderwork/go-infra/buffer/ringpool"
)

// RingBuffer is the elastic wrapper of ring.Buffer.
// It lazily acquires a ring buffer from the pool on first write
// and automatically returns it when the buffer becomes empty.
type RingBuffer struct {
	rb *ring.Buffer
}

func (b *RingBuffer) instance() *ring.Buffer {
	if b.rb == nil {
		b.rb = rbPool.Get()
	}

	return b.rb
}

// Done checks and returns the internal ring-buffer to the pool.
// The RingBuffer must not be used after calling Done.
func (b *RingBuffer) Done() {
	if b.rb != nil {
		rbPool.Put(b.rb)
		b.rb = nil
	}
}

func (b *RingBuffer) done() {
	if b.rb != nil && b.rb.IsEmpty() {
		rbPool.Put(b.rb)
		b.rb = nil
	}
}

// Peek returns the next n bytes without advancing the read pointer.
// It returns all bytes when n <= 0.
func (b *RingBuffer) Peek(n int) (head []byte, tail []byte) {
	if b.rb == nil {
		return nil, nil
	}
	return b.rb.Peek(n)
}

// Discard skips the next n bytes by advancing the read pointer.
func (b *RingBuffer) Discard(n int) (int, error) {
	if b.rb == nil {
		return 0, ring.ErrIsEmpty
	}

	defer b.done()
	return b.rb.Discard(n)
}

// Read reads up to len(p) bytes into p. It returns the number of bytes read
// (0 <= n <= len(p)) and any error encountered.
//
// Even if Read returns n < len(p), it may use all of p as scratch space during the call.
// If some data is available but not len(p) bytes, Read conventionally returns what is
// available instead of waiting for more.
//
// When Read encounters an error or end-of-file condition after successfully reading
// n > 0 bytes, it returns the number of bytes read. It may return the (non-nil) error
// from the same call or return the error (and n == 0) from a subsequent call.
//
// Callers should always process the n > 0 bytes returned before considering the error err.
// Doing so correctly handles I/O errors that happen after reading some bytes and also
// both of the allowed EOF behaviors.
func (b *RingBuffer) Read(p []byte) (int, error) {
	if b.rb == nil {
		return 0, ring.ErrIsEmpty
	}

	defer b.done()
	return b.rb.Read(p)
}

// ReadByte reads and returns the next byte from the input, or ErrIsEmpty.
func (b *RingBuffer) ReadByte() (byte, error) {
	if b.rb == nil {
		return 0, ring.ErrIsEmpty
	}

	defer b.done()
	return b.rb.ReadByte()
}

// Write writes len(p) bytes from p to the underlying buffer.
// It returns the number of bytes written (n == len(p) > 0) and any error
// encountered that caused the write to stop early.
//
// If the length of p is greater than the writable capacity, the buffer
// will grow to accommodate the data.
//
// Write must not modify the slice data, even temporarily.
func (b *RingBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return b.instance().Write(p)
}

// WriteByte writes one byte into the buffer.
func (b *RingBuffer) WriteByte(c byte) error {
	return b.instance().WriteByte(c)
}

// Buffered returns the number of bytes available to read.
func (b *RingBuffer) Buffered() int {
	if b.rb == nil {
		return 0
	}
	return b.rb.Buffered()
}

// Len returns the length of the underlying buffer.
func (b *RingBuffer) Len() int {
	if b.rb == nil {
		return 0
	}
	return b.rb.Len()
}

// Cap returns the capacity of the underlying buffer.
func (b *RingBuffer) Cap() int {
	if b.rb == nil {
		return 0
	}
	return b.rb.Cap()
}

// Available returns the number of bytes available to write.
func (b *RingBuffer) Available() int {
	if b.rb == nil {
		return 0
	}
	return b.rb.Available()
}

// WriteString writes the contents of the string s to the buffer.
func (b *RingBuffer) WriteString(s string) (int, error) {
	if len(s) == 0 {
		return 0, nil
	}
	return b.instance().WriteString(s)
}

// Bytes returns all available read bytes. It does not move the read pointer
// and only copies the available data into a new slice.
func (b *RingBuffer) Bytes() []byte {
	if b.rb == nil {
		return nil
	}
	return b.rb.Bytes()
}

// ReadFrom implements io.ReaderFrom.
func (b *RingBuffer) ReadFrom(r io.Reader) (int64, error) {
	return b.instance().ReadFrom(r)
}

// WriteTo implements io.WriterTo.
func (b *RingBuffer) WriteTo(w io.Writer) (int64, error) {
	if b.rb == nil {
		return 0, ring.ErrIsEmpty
	}

	defer b.done()
	return b.instance().WriteTo(w)
}

// IsFull reports whether the ring buffer is full.
func (b *RingBuffer) IsFull() bool {
	if b.rb == nil {
		return false
	}
	return b.rb.IsFull()
}

// IsEmpty reports whether the ring buffer is empty.
func (b *RingBuffer) IsEmpty() bool {
	if b.rb == nil {
		return true
	}
	return b.rb.IsEmpty()
}

// Reset resets the read pointer and write pointer to zero.
func (b *RingBuffer) Reset() {
	if b.rb == nil {
		return
	}
	b.rb.Reset()
}

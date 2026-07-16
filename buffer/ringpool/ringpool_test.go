package ringpool

import (
	"testing"
)

func TestDefaultPool(t *testing.T) {
	rb := Get()
	if rb == nil {
		t.Fatal("Get() returned nil")
	}
	if !rb.IsEmpty() {
		t.Fatal("expected empty buffer")
	}

	data := []byte("hello world")
	n, err := rb.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Fatalf("Write returned %d, expected %d", n, len(data))
	}

	Put(rb)

	rb2 := Get()
	if rb2 == nil {
		t.Fatal("Get() returned nil after Put")
	}
	if !rb2.IsEmpty() {
		t.Fatal("expected buffer to be reset after Put")
	}
	Put(rb2)
}

func TestCustomPool(t *testing.T) {
	var p Pool

	rb := p.Get()
	if rb == nil {
		t.Fatal("Pool.Get() returned nil")
	}

	data := []byte("test data")
	_, _ = rb.Write(data)

	p.Put(rb)

	rb2 := p.Get()
	if rb2 == nil {
		t.Fatal("Pool.Get() returned nil after Put")
	}
	if !rb2.IsEmpty() {
		t.Fatal("expected buffer to be reset after Put")
	}
	p.Put(rb2)
}

func TestIndex(t *testing.T) {
	tests := []struct {
		n   int
		idx int
	}{
		{0, 0},
		{1, 0},
		{64, 0},
		{65, 1},
		{128, 1},
		{129, 2},
	}
	for _, tt := range tests {
		got := index(tt.n)
		if got != tt.idx {
			t.Errorf("index(%d) = %d, want %d", tt.n, got, tt.idx)
		}
	}
}

func TestPoolCalibrate(t *testing.T) {
	var p Pool

	for i := 0; i < 50000; i++ {
		rb := ringBufferOfSize(128)
		p.Put(rb)
	}

	rb := p.Get()
	if rb == nil {
		t.Fatal("Pool.Get() returned nil")
	}
	p.Put(rb)
}

func ringBufferOfSize(size int) *RingBuffer {
	rb := Get()
	buf := make([]byte, size)
	_, _ = rb.Write(buf)
	return rb
}

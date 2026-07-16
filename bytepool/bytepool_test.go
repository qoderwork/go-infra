package bytepool

import (
	"testing"
)

func TestGetPut(t *testing.T) {
	b := Get(1024)
	if len(b) != 1024 {
		t.Fatalf("len(Get(1024)) = %d, want 1024", len(b))
	}
	if cap(b) < 1024 {
		t.Fatalf("cap(Get(1024)) = %d, want >= 1024", cap(b))
	}
	Put(b)
}

func TestGetZero(t *testing.T) {
	b := Get(0)
	if b != nil {
		t.Fatalf("Get(0) = %v, want nil", b)
	}
}

func TestGetNegative(t *testing.T) {
	b := Get(-1)
	if b != nil {
		t.Fatalf("Get(-1) = %v, want nil", b)
	}
}

func TestPutNil(t *testing.T) {
	Put(nil) // should not panic
}

func TestPutEmpty(t *testing.T) {
	Put([]byte{}) // should not panic
}

func TestPoolReuse(t *testing.T) {
	var p Pool
	b := p.Get(512)
	p.Put(b)
	b2 := p.Get(512)
	// Should reuse the same underlying array (same capacity class).
	if cap(b2) < 512 {
		t.Fatalf("cap(b2) = %d, want >= 512", cap(b2))
	}
}

func TestSizeClasses(t *testing.T) {
	tests := []struct {
		size int
		cap  int // expected capacity (power of two)
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{1023, 1024},
		{1024, 1024},
		{1025, 2048},
	}
	var p Pool
	for _, tt := range tests {
		b := p.Get(tt.size)
		if cap(b) != tt.cap {
			t.Errorf("Get(%d): cap = %d, want %d", tt.size, cap(b), tt.cap)
		}
		p.Put(b)
	}
}

func BenchmarkGetPut(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := Get(4096)
		Put(buf)
	}
}
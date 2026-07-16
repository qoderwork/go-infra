package mathutil

import "testing"

func TestIsPowerOfTwo(t *testing.T) {
	tests := []struct {
		n    int
		want bool
	}{
		{0, false},
		{1, true},
		{2, true},
		{3, false},
		{4, true},
		{5, false},
		{8, true},
		{-1, false},
	}
	for _, tt := range tests {
		if got := IsPowerOfTwo(tt.n); got != tt.want {
			t.Errorf("IsPowerOfTwo(%d) = %v, want %v", tt.n, got, tt.want)
		}
	}
}

func TestCeilToPowerOfTwo(t *testing.T) {
	tests := []struct {
		n, want int
	}{
		{0, 2},
		{1, 2},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{63, 64},
		{64, 64},
		{100, 128},
		{1024, 1024},
	}
	for _, tt := range tests {
		if got := CeilToPowerOfTwo(tt.n); got != tt.want {
			t.Errorf("CeilToPowerOfTwo(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestFloorToPowerOfTwo(t *testing.T) {
	tests := []struct {
		n, want int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 2},
		{4, 4},
		{5, 4},
		{63, 32},
		{64, 64},
		{100, 64},
		{1024, 1024},
	}
	for _, tt := range tests {
		if got := FloorToPowerOfTwo(tt.n); got != tt.want {
			t.Errorf("FloorToPowerOfTwo(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestClosestPowerOfTwo(t *testing.T) {
	tests := []struct {
		n, want int
	}{
		{1, 1},    // 1 is closer to 1 (2^0) than 2 (2^1)
		{2, 2},    // 2 is a power of two
		{3, 4},    // 3-2=1, 4-3=1 → equidistant, picks larger (4)
		{4, 4},    // 4 is a power of two
		{5, 4},    // 5-4=1, 8-5=3 → 4
		{6, 8},    // 6-4=2, 8-6=2 → equidistant, picks larger (8)
		{10, 8},   // 10-8=2, 16-10=6 → 8
		{12, 16},  // 12-8=4, 16-12=4 → equidistant, picks larger (16)
		{14, 16},  // 14-8=6, 16-14=2 → 16
	}
	for _, tt := range tests {
		if got := ClosestPowerOfTwo(tt.n); got != tt.want {
			t.Errorf("ClosestPowerOfTwo(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestCeilToPowerOfTwoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("CeilToPowerOfTwo should panic for too-large argument")
		}
	}()
	// On 64-bit, maxintHeadBit = 1 << 62. Values > maxintHeadBit with bit 62 set should panic.
	CeilToPowerOfTwo((1 << 62) + 1)
}
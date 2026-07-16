// Package mathutil provides fast math utility functions.
//
// Derived from gnet/pkg/math (https://github.com/panjf2000/gnet),
// licensed under Apache 2.0.
package mathutil

import "math/bits"

const (
	bitSize       = 32 << (^uint(0) >> 63)
	maxintHeadBit = 1 << (bitSize - 2)
)

// IsPowerOfTwo reports whether n is a power of two.
func IsPowerOfTwo(n int) bool {
	return n > 0 && n&(n-1) == 0
}

// CeilToPowerOfTwo returns n if it is a power-of-two,
// otherwise the next-highest power-of-two.
//
// Panics if n is too large to be rounded up to a power of two
// within the int range.
func CeilToPowerOfTwo(n int) int {
	if n&maxintHeadBit != 0 && n > maxintHeadBit {
		panic("mathutil: argument is too large")
	}

	if n <= 2 {
		return 2
	}
	return 1 << bits.Len(uint(n-1))
}

// FloorToPowerOfTwo returns n if it is a power-of-two,
// otherwise the next-lowest power-of-two.
func FloorToPowerOfTwo(n int) int {
	if n <= 2 {
		return n
	}

	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16

	return n - (n >> 1)
}

// ClosestPowerOfTwo returns n if it is a power-of-two,
// otherwise the closest power-of-two.
func ClosestPowerOfTwo(n int) int {
	next := CeilToPowerOfTwo(n)
	if prev := next / 2; (n - prev) < (next - n) {
		next = prev
	}
	return next
}
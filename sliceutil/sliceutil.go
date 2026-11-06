// Package sliceutil provides slice manipulation utilities.
//
// Value semantics: every function that returns a slice (Remove, RemoveAt,
// Insert, RemoveAll, Union, Unique, Filter, Map, ...) never mutates its input.
// When it actually changes the contents it allocates a new backing array, so
// the result does not alias the caller's slice; when nothing changes (a missing
// value, an out-of-range index) the input itself may be returned. Reverse is
// the single intentional in-place operation (it returns nothing).
package sliceutil

// Unique returns a new slice with duplicate values removed.
// Order is preserved. Uses O(n) time with O(n) extra space.
func Unique[T comparable](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[T]bool)
	result := make([]T, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// IsUnique reports whether all elements in s are distinct.
func IsUnique[T comparable](s []T) bool {
	seen := make(map[T]struct{}, len(s))
	for _, v := range s {
		if _, ok := seen[v]; ok {
			return false
		}
		seen[v] = struct{}{}
	}
	return true
}

// UniqueBy returns a new slice with duplicate values removed, where the
// uniqueness key is derived by fn. Order is preserved and the first
// occurrence of each key is kept.
func UniqueBy[T any, K comparable](s []T, fn func(T) K) []T {
	if len(s) == 0 {
		return nil
	}
	seen := make(map[K]struct{}, len(s))
	result := make([]T, 0, len(s))
	for _, v := range s {
		key := fn(v)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}

// Intersect returns a new slice containing elements that are in both slices.
// Order follows the first slice. Duplicates in the result are removed.
func Intersect[T comparable](a, b []T) []T {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	bset := make(map[T]bool, len(b))
	for _, v := range b {
		bset[v] = true
	}
	seen := make(map[T]bool)
	result := make([]T, 0, len(a))
	for _, v := range a {
		if bset[v] && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// Difference returns a new slice containing elements that are in a but not in b.
// Order follows the first slice. Duplicates in the result are removed.
func Difference[T comparable](a, b []T) []T {
	if len(a) == 0 {
		return nil
	}
	if len(b) == 0 {
		return Unique(a)
	}
	bset := make(map[T]bool, len(b))
	for _, v := range b {
		bset[v] = true
	}
	seen := make(map[T]bool)
	result := make([]T, 0, len(a))
	for _, v := range a {
		if !bset[v] && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// Union returns a new slice containing elements that are in either slice.
// Duplicates are removed. Neither input is mutated.
func Union[T comparable](a, b []T) []T {
	combined := make([]T, 0, len(a)+len(b))
	combined = append(combined, a...)
	combined = append(combined, b...)
	return Unique(combined)
}

// Contains returns true if the slice contains the value.
func Contains[T comparable](s []T, v T) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// Index returns the index of the first occurrence of v in s, or -1 if not found.
func Index[T comparable](s []T, v T) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// LastIndex returns the index of the last occurrence of v in s, or -1 if not found.
func LastIndex[T comparable](s []T, v T) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == v {
			return i
		}
	}
	return -1
}

// Remove returns a new slice with the first occurrence of v removed. The input
// slice is not mutated. If v is absent the input is returned unchanged.
func Remove[T comparable](s []T, v T) []T {
	i := Index(s, v)
	if i == -1 {
		return s
	}
	out := make([]T, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}

// RemoveAll removes all occurrences of v from the slice.
func RemoveAll[T comparable](s []T, v T) []T {
	result := make([]T, 0, len(s))
	for _, x := range s {
		if x != v {
			result = append(result, x)
		}
	}
	return result
}

// RemoveAt returns a new slice with the element at index i removed. The input
// slice is not mutated. If i is out of range the input is returned unchanged.
func RemoveAt[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return s
	}
	out := make([]T, 0, len(s)-1)
	out = append(out, s[:i]...)
	out = append(out, s[i+1:]...)
	return out
}

// Insert returns a new slice with v placed at index i. The input slice is not
// mutated. A negative index inserts at the front; an index >= len(s) appends.
func Insert[T any](s []T, i int, v T) []T {
	if i < 0 {
		i = 0
	}
	out := make([]T, 0, len(s)+1)
	if i >= len(s) {
		out = append(out, s...)
		return append(out, v)
	}
	out = append(out, s[:i]...)
	out = append(out, v)
	out = append(out, s[i:]...)
	return out
}

// Reverse reverses the slice in place.
func Reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Clone returns a shallow copy of the slice.
func Clone[T any](s []T) []T {
	if s == nil {
		return nil
	}
	return append([]T{}, s...)
}

// Map returns a new slice with the results of applying fn to each element.
func Map[T, U any](s []T, fn func(T) U) []U {
	result := make([]U, len(s))
	for i, v := range s {
		result[i] = fn(v)
	}
	return result
}

// Filter returns a new slice containing only elements where fn returns true.
func Filter[T any](s []T, fn func(T) bool) []T {
	result := make([]T, 0, len(s))
	for _, v := range s {
		if fn(v) {
			result = append(result, v)
		}
	}
	return result
}

// Reduce reduces the slice to a single value using fn.
func Reduce[T, U any](s []T, init U, fn func(U, T) U) U {
	result := init
	for _, v := range s {
		result = fn(result, v)
	}
	return result
}

// All returns true if fn returns true for all elements.
func All[T any](s []T, fn func(T) bool) bool {
	for _, v := range s {
		if !fn(v) {
			return false
		}
	}
	return true
}

// Any returns true if fn returns true for at least one element.
func Any[T any](s []T, fn func(T) bool) bool {
	for _, v := range s {
		if fn(v) {
			return true
		}
	}
	return false
}

// ForEach calls fn for each element in the slice.
func ForEach[T any](s []T, fn func(T)) {
	for _, v := range s {
		fn(v)
	}
}

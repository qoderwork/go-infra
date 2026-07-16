// Package sliceutil provides slice manipulation utilities.
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
// Duplicates are removed.
func Union[T comparable](a, b []T) []T {
	return Unique(append(a, b...))
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

// Remove removes the first occurrence of v from the slice.
func Remove[T comparable](s []T, v T) []T {
	i := Index(s, v)
	if i == -1 {
		return s
	}
	return append(s[:i], s[i+1:]...)
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

// RemoveAt removes the element at the given index.
// Returns nil if index is out of bounds.
func RemoveAt[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return s
	}
	return append(s[:i], s[i+1:]...)
}

// Insert inserts the value at the given index.
// If index is out of bounds, appends to the end.
func Insert[T any](s []T, i int, v T) []T {
	if i < 0 {
		i = 0
	}
	if i >= len(s) {
		return append(s, v)
	}
	s = append(s[:i+1], s[i:]...)
	s[i] = v
	return s
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
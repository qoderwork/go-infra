package sliceutil

import (
	"testing"

	"github.com/qoderwork/go-infra/assert"
)

// TestValueHelpersDoNotMutateInput guards the copy-on-write behavior: Remove,
// RemoveAt, Insert and Union must not modify the caller's slice or share its
// backing array. Each input is built with spare capacity, which the old in-place
// implementations silently clobbered via append.
func TestValueHelpersDoNotMutateInput(t *testing.T) {
	// Remove
	s := append(make([]int, 0, 8), 1, 2, 3, 2)
	snapshot := Clone(s)
	got := Remove(s, 2)
	assert.Equal(t, []int{1, 3, 2}, got)
	assert.Equal(t, snapshot, s, "Remove must not mutate input")

	// RemoveAt
	s = append(make([]int, 0, 8), 1, 2, 3)
	snapshot = Clone(s)
	got = RemoveAt(s, 1)
	assert.Equal(t, []int{1, 3}, got)
	assert.Equal(t, snapshot, s, "RemoveAt must not mutate input")

	// Insert
	s = append(make([]int, 0, 8), 1, 3)
	snapshot = Clone(s)
	got = Insert(s, 1, 2)
	assert.Equal(t, []int{1, 2, 3}, got)
	assert.Equal(t, snapshot, s, "Insert must not mutate input")

	// Union
	a := append(make([]int, 0, 8), 1, 2, 3)
	b := append(make([]int, 0, 8), 3, 4, 5)
	aSnapshot, bSnapshot := Clone(a), Clone(b)
	got = Union(a, b)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, got)
	assert.Equal(t, aSnapshot, a, "Union must not mutate a")
	assert.Equal(t, bSnapshot, b, "Union must not mutate b")

	// Result must not alias the input backing array.
	got[0] = 999
	assert.Equal(t, aSnapshot, a, "Union result must not alias input")
}

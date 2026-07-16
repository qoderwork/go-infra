package sliceutil

import "testing"

func TestUnique(t *testing.T) {
	tests := []struct {
		in, want []int
	}{
		{[]int{1, 2, 3, 2, 1}, []int{1, 2, 3}},
		{[]int{1, 1, 1}, []int{1}},
		{[]int{}, nil},
		{nil, nil},
	}
	for _, tt := range tests {
		if got := Unique(tt.in); !equal(got, tt.want) {
			t.Errorf("Unique(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestIntersect(t *testing.T) {
	a := []int{1, 2, 3, 4}
	b := []int{3, 4, 5, 6}
	want := []int{3, 4}
	if got := Intersect(a, b); !equal(got, want) {
		t.Errorf("Intersect = %v, want %v", got, want)
	}
}

func TestDifference(t *testing.T) {
	a := []int{1, 2, 3, 4}
	b := []int{3, 4, 5, 6}
	want := []int{1, 2}
	if got := Difference(a, b); !equal(got, want) {
		t.Errorf("Difference = %v, want %v", got, want)
	}
}

func TestUnion(t *testing.T) {
	a := []int{1, 2, 3}
	b := []int{3, 4, 5}
	want := []int{1, 2, 3, 4, 5}
	if got := Union(a, b); !equal(got, want) {
		t.Errorf("Union = %v, want %v", got, want)
	}
}

func TestContains(t *testing.T) {
	s := []int{1, 2, 3}
	if !Contains(s, 2) {
		t.Error("Contains(2) = false, want true")
	}
	if Contains(s, 4) {
		t.Error("Contains(4) = true, want false")
	}
}

func TestIndex(t *testing.T) {
	s := []int{1, 2, 3, 2}
	if got := Index(s, 2); got != 1 {
		t.Errorf("Index(2) = %d, want 1", got)
	}
	if got := Index(s, 4); got != -1 {
		t.Errorf("Index(4) = %d, want -1", got)
	}
}

func TestLastIndex(t *testing.T) {
	s := []int{1, 2, 3, 2}
	if got := LastIndex(s, 2); got != 3 {
		t.Errorf("LastIndex(2) = %d, want 3", got)
	}
}

func TestRemove(t *testing.T) {
	s := []int{1, 2, 3, 2}
	if got := Remove(s, 2); !equal(got, []int{1, 3, 2}) {
		t.Errorf("Remove = %v, want [1, 3, 2]", got)
	}
}

func TestRemoveAll(t *testing.T) {
	s := []int{1, 2, 3, 2}
	if got := RemoveAll(s, 2); !equal(got, []int{1, 3}) {
		t.Errorf("RemoveAll = %v, want [1, 3]", got)
	}
}

func TestRemoveAt(t *testing.T) {
	s := []int{1, 2, 3}
	if got := RemoveAt(s, 1); !equal(got, []int{1, 3}) {
		t.Errorf("RemoveAt = %v, want [1, 3]", got)
	}
	// Test out of bounds.
	s2 := []int{1, 2, 3}
	if got := RemoveAt(s2, 10); !equal(got, []int{1, 2, 3}) {
		t.Errorf("RemoveAt(out of bounds) = %v, want original", got)
	}
}

func TestInsert(t *testing.T) {
	s := []int{1, 3}
	if got := Insert(s, 1, 2); !equal(got, []int{1, 2, 3}) {
		t.Errorf("Insert = %v, want [1, 2, 3]", got)
	}
}

func TestReverse(t *testing.T) {
	s := []int{1, 2, 3}
	Reverse(s)
	if !equal(s, []int{3, 2, 1}) {
		t.Errorf("Reverse = %v, want [3, 2, 1]", s)
	}
}

func TestClone(t *testing.T) {
	s := []int{1, 2, 3}
	c := Clone(s)
	if !equal(c, s) {
		t.Errorf("Clone = %v, want %v", c, s)
	}
	c[0] = 99
	if s[0] == 99 {
		t.Error("Clone did not copy, original modified")
	}
}

func TestMap(t *testing.T) {
	s := []int{1, 2, 3}
	got := Map(s, func(x int) int { return x * 2 })
	want := []int{2, 4, 6}
	if !equal(got, want) {
		t.Errorf("Map = %v, want %v", got, want)
	}
}

func TestFilter(t *testing.T) {
	s := []int{1, 2, 3, 4, 5}
	got := Filter(s, func(x int) bool { return x%2 == 0 })
	want := []int{2, 4}
	if !equal(got, want) {
		t.Errorf("Filter = %v, want %v", got, want)
	}
}

func TestReduce(t *testing.T) {
	s := []int{1, 2, 3, 4}
	got := Reduce(s, 0, func(sum, x int) int { return sum + x })
	if got != 10 {
		t.Errorf("Reduce = %d, want 10", got)
	}
}

func TestAll(t *testing.T) {
	s := []int{2, 4, 6}
	if !All(s, func(x int) bool { return x%2 == 0 }) {
		t.Error("All even = false, want true")
	}
}

func TestAny(t *testing.T) {
	s := []int{1, 2, 3}
	if !Any(s, func(x int) bool { return x == 2 }) {
		t.Error("Any(2) = false, want true")
	}
}

func equal[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
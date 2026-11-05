package queue

import "testing"

// TestNewCirQueueRejectsZeroSize: size=0 used to build a zero-length slice and
// panic later with "index out of range" on the first Push (and divide by zero
// in the modulo operations).
func TestNewCirQueueRejectsZeroSize(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewCirQueue(0) should panic with a clear message")
		}
	}()
	NewCirQueue[int](0)
}

func TestCirQueueSmallestSize(t *testing.T) {
	q := NewCirQueue[int](1)
	q.Push(7)
	got := q.Range()
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("Range = %v, want [7]", got)
	}
}

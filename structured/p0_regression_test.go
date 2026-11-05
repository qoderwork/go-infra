package structured

import (
	"testing"
	"time"
)

// TestStreamDefaultQueueDoesNotDeadlock reproduces the old behavior: with no
// WithMaxGoroutines the callback queue had capacity 1, so the third Go()
// blocked forever when tasks waited for the submitter to continue.
func TestStreamDefaultQueueDoesNotDeadlock(t *testing.T) {
	start := make(chan struct{})
	s := NewStream() // deliberately no WithMaxGoroutines
	submitted := make(chan struct{})

	go func() {
		for i := 0; i < 20; i++ {
			s.Go(func() Callback {
				<-start
				return func() {}
			})
		}
		close(submitted)
	}()

	select {
	case <-submitted:
	case <-time.After(2 * time.Second):
		close(start)
		t.Fatal("default Stream deadlocked while submitting blocking tasks")
	}
	close(start)
	s.Wait()
}

// TestStreamDefaultSelfCompletingTasks covers the common case to make sure the
// queue floor did not change ordering or completion semantics.
func TestStreamDefaultSelfCompletingTasks(t *testing.T) {
	s := NewStream()
	var order []int
	for i := 0; i < 6; i++ {
		i := i
		s.Go(func() Callback {
			return func() { order = append(order, i) }
		})
	}
	s.Wait()
	for i, v := range order {
		if v != i {
			t.Fatalf("callbacks out of order: %v", order)
		}
	}
	if len(order) != 6 {
		t.Fatalf("got %d callbacks, want 6", len(order))
	}
}

// TestPoolReuseAfterPanicDoesNotReplay: Wait() resets initOnce but used to
// leave the WaitGroup's Catcher holding the first batch's panic.
func TestPoolReuseAfterPanicDoesNotReplay(t *testing.T) {
	p := NewPool()
	p.Go(func() { panic("boom") })
	func() {
		defer func() {
			if recover() == nil {
				t.Error("first Wait should propagate the panic")
			}
		}()
		p.Wait()
	}()

	p.Go(func() {})
	stale := false
	func() {
		defer func() {
			if recover() != nil {
				stale = true
			}
		}()
		p.Wait()
	}()
	if stale {
		t.Fatal("second Wait replayed a stale panic from the previous batch")
	}
}

// TestErrorPoolReuseAfterPanicDoesNotReplay covers the derived pool family,
// which all delegate to Pool.Wait().
func TestErrorPoolReuseAfterPanicDoesNotReplay(t *testing.T) {
	p := NewPool().WithErrors()
	p.Go(func() error { panic("boom") })
	func() {
		defer func() { _ = recover() }()
		_ = p.Wait()
	}()

	p.Go(func() error { return nil })
	stale := false
	func() {
		defer func() {
			if recover() != nil {
				stale = true
			}
		}()
		_ = p.Wait()
	}()
	if stale {
		t.Fatal("ErrorPool replayed a stale panic")
	}
}

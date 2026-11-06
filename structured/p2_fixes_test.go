package structured

import (
	"testing"
	"time"
)

// TestBackoffDelayNeverExceedsMaxDelay guards the jitter-order fix: the delay
// must be clamped AFTER jitter so it never exceeds MaxDelay. Pure and
// deterministic (no sleeping).
func TestBackoffDelayNeverExceedsMaxDelay(t *testing.T) {
	cfg := RetryConfig{Attempts: 50, BaseDelay: time.Millisecond, MaxDelay: 50 * time.Millisecond}
	for attempt := 0; attempt < 50; attempt++ {
		for i := 0; i < 2000; i++ { // sample the jitter many times
			d := backoffDelay(cfg, attempt)
			if d > cfg.MaxDelay {
				t.Fatalf("attempt %d: delay %v exceeds MaxDelay %v", attempt, d, cfg.MaxDelay)
			}
			if d <= 0 {
				t.Fatalf("attempt %d: delay %v must be positive", attempt, d)
			}
		}
	}
}

// TestBackoffDelayStillJittersAtCap guards the opposite failure mode: once the
// exponential delay reaches MaxDelay, a one-sided positive jitter is always
// clamped back to the cap, so every retry would sleep exactly MaxDelay and
// retry in lockstep. Symmetric jitter must still produce variation at the cap.
func TestBackoffDelayStillJittersAtCap(t *testing.T) {
	cfg := RetryConfig{BaseDelay: time.Second, MaxDelay: time.Second}
	for i := 0; i < 200; i++ {
		if d := backoffDelay(cfg, 0); d < cfg.MaxDelay {
			return
		}
	}
	t.Fatal("delay at MaxDelay is constant; jitter was clamped away")
}

// TestWaitGroupConcurrentWaitersAllSeePanic: WaitGroup documents
// sync.WaitGroup-like semantics, so more than one goroutine may Wait. The
// Catcher must stay persistent until an explicit Pool-level reset between
// batches; clearing it inside Wait would let later Waiters silently miss the
// panic.
func TestWaitGroupConcurrentWaitersAllSeePanic(t *testing.T) {
	wg := NewWaitGroup()
	wg.Go(func() { panic("boom") })

	const waiters = 4
	start := make(chan struct{})
	results := make(chan *Recovered, waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			<-start
			results <- wg.WaitAndRecover()
		}()
	}
	close(start)
	for i := 0; i < waiters; i++ {
		if got := <-results; got == nil {
			t.Fatal("a concurrent Waiter missed the panic")
		}
	}
}

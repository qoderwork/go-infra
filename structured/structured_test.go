package structured

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitGroupWaitsForAllGoroutines(t *testing.T) {
	var wg WaitGroup
	var count atomic.Int64
	for i := 0; i < 20; i++ {
		wg.Go(func() { count.Add(1) })
	}
	wg.Wait()
	if got := count.Load(); got != 20 {
		t.Fatalf("expected 20 completed tasks, got %d", got)
	}
}

func TestWaitGroupPropagatesPanic(t *testing.T) {
	var wg WaitGroup
	wg.Go(func() { panic("boom") })
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Wait did not propagate the child panic")
		}
		rec, ok := r.(*Recovered)
		if !ok {
			t.Fatalf("expected *Recovered, got %T", r)
		}
		if rec.Value != "boom" {
			t.Fatalf("unexpected panic value: %v", rec.Value)
		}
		if !strings.Contains(string(rec.Stack), "TestWaitGroupPropagatesPanic") {
			t.Fatal("recovered stack lost the child goroutine frames")
		}
	}()
	wg.Wait()
}

func TestWaitGroupWaitAndRecover(t *testing.T) {
	var wg WaitGroup
	wg.Go(func() { panic("recoverable") })
	rec := wg.WaitAndRecover()
	if rec == nil {
		t.Fatal("expected a recovered panic")
	}
	if err := rec.AsError(); err == nil || !strings.Contains(err.Error(), "recoverable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTryCatchesPanic(t *testing.T) {
	rec := Try(func() { panic(42) })
	if rec == nil || rec.Value != 42 {
		t.Fatalf("unexpected recovered value: %+v", rec)
	}
	if got := Try(func() {}); got != nil {
		t.Fatalf("expected nil for a function that does not panic, got %+v", got)
	}
}

func TestPoolLimitsConcurrentGoroutines(t *testing.T) {
	p := NewPool().WithMaxGoroutines(2)
	var current, peak atomic.Int64
	for i := 0; i < 30; i++ {
		p.Go(func() {
			n := current.Add(1)
			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			current.Add(-1)
		})
	}
	p.Wait()
	if peak.Load() > 2 {
		t.Fatalf("pool exceeded its goroutine limit: peak=%d", peak.Load())
	}
}

func TestErrorPoolJoinsErrors(t *testing.T) {
	errA := errors.New("a")
	errB := errors.New("b")
	p := NewPool().WithErrors().WithMaxGoroutines(2)
	p.Go(func() error { return errA })
	p.Go(func() error { return errB })
	p.Go(func() error { return nil })
	err := p.Wait()
	if !errors.Is(err, errA) || !errors.Is(err, errB) {
		t.Fatalf("errors were not joined: %v", err)
	}
}

func TestContextPoolCancelsSiblingsOnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := NewPool().WithContext(ctx).WithCancelOnError().WithFirstError().WithMaxGoroutines(2)
	firstErr := errors.New("first failure")
	started := make(chan struct{})
	p.Go(func(context.Context) error {
		<-started
		return firstErr
	})
	p.Go(func(c context.Context) error {
		close(started)
		select {
		case <-c.Done():
			return nil
		case <-time.After(time.Second):
			return errors.New("sibling was not canceled")
		}
	})
	if err := p.Wait(); !errors.Is(err, firstErr) {
		t.Fatalf("expected the first error, got %v", err)
	}
}

func TestResultPoolKeepsSubmissionOrder(t *testing.T) {
	p := NewResultPool[int]().WithMaxGoroutines(3)
	for i := 0; i < 25; i++ {
		i := i
		p.Go(func() int {
			time.Sleep(time.Duration(25-i%5) * time.Microsecond)
			return i
		})
	}
	got := p.Wait()
	if len(got) != 25 {
		t.Fatalf("expected 25 results, got %d", len(got))
	}
	for i, v := range got {
		if v != i {
			t.Fatalf("results out of order at %d: %d", i, v)
		}
	}
}

func TestResultErrorPoolFiltersErroredTasks(t *testing.T) {
	p := NewResultPool[string]().WithErrors().WithFirstError()
	p.Go(func() (string, error) { return "ok", nil })
	p.Go(func() (string, error) { return "bad", errors.New("failed") })
	results, err := p.Wait()
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(results) != 1 || results[0] != "ok" {
		t.Fatalf("errored result should be filtered out, got %v", results)
	}
}

func TestIterForEachAndMap(t *testing.T) {
	values := []int{1, 2, 3, 4, 5}
	ForEach(values, func(v *int) { *v *= 2 })
	for i, v := range values {
		if v != (i+1)*2 {
			t.Fatalf("ForEach did not mutate in place: %v", values)
		}
	}
	mapped := Map([]int{1, 2, 3}, func(v *int) string { return fmt.Sprint(*v) })
	if strings.Join(mapped, ",") != "1,2,3" {
		t.Fatalf("Map result out of order: %v", mapped)
	}
	_, err := MapErr([]int{1, 2}, func(v *int) (int, error) {
		return *v, errors.New("map failed")
	})
	if err == nil || !strings.Contains(err.Error(), "map failed") {
		t.Fatalf("MapErr did not join errors: %v", err)
	}
}

func TestStreamCallbacksRunInSubmissionOrder(t *testing.T) {
	s := NewStream().WithMaxGoroutines(4)
	var order []int
	for i := 0; i < 20; i++ {
		i := i
		s.Go(func() Callback {
			time.Sleep(time.Duration(20-i%4) * time.Microsecond)
			return func() { order = append(order, i) }
		})
	}
	s.Wait()
	for i, v := range order {
		if v != i {
			t.Fatalf("callbacks out of order at %d: %d (full: %v)", i, v, order)
		}
	}
}

func TestCollectorAllAndIter(t *testing.T) {
	// The buffer must hold every outstanding Submit: there is no consumer
	// running until All() is called.
	c := NewCollector[int](context.Background(), 8)
	for i := 0; i < 5; i++ {
		c.Submit(i)
	}
	c.Close()
	c.Close() // idempotent
	values, err := c.All()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 5 || values[4] != 4 {
		t.Fatalf("unexpected values: %v", values)
	}

	c2 := NewCollector[string](context.Background(), 2)
	c2.Submit("a")
	c2.Submit("b")
	c2.Close()
	var seen []string
	for v, err := range c2.Iter() {
		if err != nil {
			t.Fatalf("unexpected iteration error: %v", err)
		}
		seen = append(seen, v)
	}
	if strings.Join(seen, "") != "ab" {
		t.Fatalf("unexpected iteration: %v", seen)
	}
}

func TestCollectorHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := NewCollector[int](ctx, 1)
	c.Submit(1)
	c.Close()
	cancel()
	if _, err := c.All(); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRetrySucceedsAfterFailures(t *testing.T) {
	cfg := RetryConfig{Attempts: 4, BaseDelay: time.Microsecond, MaxDelay: time.Millisecond}
	var calls int
	err := Retry(context.Background(), cfg, func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("expected success on attempt 3, got calls=%d err=%v", calls, err)
	}
}

func TestRetryExhaustionWrapsLastError(t *testing.T) {
	sentinel := errors.New("permanent")
	cfg := RetryConfig{Attempts: 3, BaseDelay: time.Microsecond, MaxDelay: time.Millisecond}
	var calls int
	err := Retry(context.Background(), cfg, func(context.Context) error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) || calls != 3 {
		t.Fatalf("expected wrapped sentinel after 3 attempts, got calls=%d err=%v", calls, err)
	}
}

func TestRetryStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := RetryConfig{Attempts: 10, BaseDelay: time.Hour, MaxDelay: time.Hour}
	var calls int
	err := Retry(ctx, cfg, func(context.Context) error {
		calls++
		cancel()
		return errors.New("keep going")
	})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("expected immediate cancellation, got calls=%d err=%v", calls, err)
	}
}

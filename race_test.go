package lifecycle

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestRace_ConcurrentStateReads verifies no data race when reading State()
// from multiple goroutines while Start/Stop are in progress.
func TestRace_ConcurrentStateReads(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	m.AddTask(NewFuncTask("t",
		func(ctx context.Context) error { time.Sleep(10 * time.Millisecond); return nil },
		func(ctx context.Context) error { time.Sleep(10 * time.Millisecond); return nil },
	))

	var wg sync.WaitGroup
	// Readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.State()
				_ = m.Reason()
				_ = m.Err()
			}
		}()
	}

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	wg.Wait()
}

// TestRace_ConcurrentAddTask verifies AddTask is safe before Start.
func TestRace_ConcurrentAddTask(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = m.AddTask(NewFuncTask("t", nil, nil))
		}(i)
	}
	wg.Wait()

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())
}

// TestRace_ConcurrentStop verifies multiple goroutines calling Stop is safe.
func TestRace_ConcurrentStop(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	m.AddTask(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Stop(context.Background())
		}()
	}
	wg.Wait()

	if m.State() != StateStopped {
		t.Fatalf("state = %s, want Stopped", m.State())
	}
}

// TestRace_ParallelExecutor verifies no race in parallel executor.
func TestRace_ParallelExecutor(t *testing.T) {
	m := NewManager(
		WithLogger(NopLogger()),
		WithExecutor(ExecutorParallel),
	)

	var count atomic.Int32
	for i := 0; i < 20; i++ {
		m.AddTask(NewFuncTask("pt",
			func(ctx context.Context) error { count.Add(1); return nil },
			func(ctx context.Context) error { count.Add(1); return nil },
		))
	}

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if count.Load() != 40 {
		t.Fatalf("expected 40 operations, got %d", count.Load())
	}
}

// TestRace_MetricsHook verifies metrics hook is safe under concurrent use.
func TestRace_MetricsHook(t *testing.T) {
	var count atomic.Int32

	m := NewManager(
		WithLogger(NopLogger()),
		WithMetricsHook(func(e MetricEvent) {
			count.Add(1)
		}),
	)
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if count.Load() == 0 {
		t.Fatal("expected metrics events, got 0")
	}
}

// TestRace_WaitAndStop verifies Wait + concurrent Stop is safe.
func TestRace_WaitAndStop(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()), WithShutdownTimeout(2*time.Second))
	m.AddTask(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.Wait(ctx)
	}()

	// Let Wait start blocking
	time.Sleep(10 * time.Millisecond)

	// Concurrent Stop and cancel
	wg.Add(1)
	go func() {
		defer wg.Done()
		cancel()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.Stop(context.Background())
	}()

	wg.Wait()
}

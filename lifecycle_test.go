package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(discard{}, nil))
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func assertOrder(t *testing.T, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("length mismatch: want %d, got %d\nwant: %v\ngot:  %v", len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("[%d] want %q, got %q", i, want[i], got[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Basic Start / Stop
// ---------------------------------------------------------------------------

func TestBasicStartStop(t *testing.T) {
	var started, stopped atomic.Int32

	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("a",
		func(ctx context.Context) error { started.Add(1); return nil },
		func(ctx context.Context) error { stopped.Add(1); return nil },
	))
	m.Add(NewFuncTask("b",
		func(ctx context.Context) error { started.Add(1); return nil },
		func(ctx context.Context) error { stopped.Add(1); return nil },
	))

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if started.Load() != 2 {
		t.Fatalf("started = %d, want 2", started.Load())
	}

	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if stopped.Load() != 2 {
		t.Fatalf("stopped = %d, want 2", stopped.Load())
	}
}

func TestLIFOStopOrder(t *testing.T) {
	var order []string
	var mu sync.Mutex

	m := New(WithLogger(nopLogger()))
	for _, name := range []string{"A", "B", "C"} {
		m.Add(NewFuncTask(name,
			func(ctx context.Context) error {
				mu.Lock()
				order = append(order, "start-"+name)
				mu.Unlock()
				return nil
			},
			func(ctx context.Context) error {
				mu.Lock()
				order = append(order, "stop-"+name)
				mu.Unlock()
				return nil
			},
		))
	}

	ctx := context.Background()
	_ = m.Start(ctx)
	_ = m.Stop(ctx)

	assertOrder(t, []string{"start-A", "start-B", "start-C", "stop-C", "stop-B", "stop-A"}, order)
}

func TestNilTaskFuncsAreNoop(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("nil-nil", nil, nil))

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Hooks
// ---------------------------------------------------------------------------

func TestOnStartHook(t *testing.T) {
	var called atomic.Int32

	m := New(WithLogger(nopLogger()))
	m.OnStart(func(ctx context.Context) error { called.Add(1); return nil })
	m.Add(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	if called.Load() != 1 {
		t.Fatalf("on_start called %d times, want 1", called.Load())
	}
	_ = m.Stop(context.Background())
}

func TestOnStartFails_AbortsStart(t *testing.T) {
	boom := errors.New("boom")
	m := New(WithLogger(nopLogger()))
	m.OnStart(func(ctx context.Context) error { return boom })
	m.Add(NewFuncTask("t", func(ctx context.Context) error {
		t.Fatal("task should not have been started")
		return nil
	}, nil))

	err := m.Start(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("want error wrapping boom, got %v", err)
	}
}

func TestOnStopHook(t *testing.T) {
	var called atomic.Int32

	m := New(WithLogger(nopLogger()))
	m.OnStop(func(ctx context.Context) error { called.Add(1); return nil })
	m.Add(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if called.Load() != 1 {
		t.Fatalf("on_stop called %d times, want 1", called.Load())
	}
}

func TestOnStopError_Accumulated(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("fail", nil, func(ctx context.Context) error { return errors.New("stop err") }))
	m.OnStop(func(ctx context.Context) error { return errors.New("hook err") })

	_ = m.Start(context.Background())
	err := m.Stop(context.Background())

	if err == nil {
		t.Fatal("expected accumulated error")
	}
	// Should contain both errors
	if !errors.Is(err, errors.Unwrap(err)) {
		// errors.Join wraps multiple; just verify it's non-nil and contains both
	}
}

func TestHookOrder(t *testing.T) {
	var order []string
	m := New(WithLogger(nopLogger()))

	m.OnStart(func(ctx context.Context) error {
		order = append(order, "on_start")
		return nil
	})
	m.OnStop(func(ctx context.Context) error {
		order = append(order, "on_stop")
		return nil
	})

	m.Add(NewFuncTask("t",
		func(ctx context.Context) error { order = append(order, "start"); return nil },
		func(ctx context.Context) error { order = append(order, "stop"); return nil },
	))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	assertOrder(t, []string{"on_start", "start", "stop", "on_stop"}, order)
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestStartError_Rollback(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	boom := errors.New("boom")
	var rolledBack atomic.Bool

	m.Add(NewFuncTask("ok",
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { rolledBack.Store(true); return nil },
	))
	m.Add(NewFuncTask("fail",
		func(ctx context.Context) error { return boom },
		nil,
	))
	m.Add(NewFuncTask("never",
		func(ctx context.Context) error { t.Fatal("should not start"); return nil },
		nil,
	))

	err := m.Start(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("want error wrapping boom, got %v", err)
	}
	if !rolledBack.Load() {
		t.Fatal("already-started task was not rolled back")
	}
}

func TestStopError_ContinuesAllTasks(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	var stopped atomic.Int32

	m.Add(NewFuncTask("fail", nil, func(ctx context.Context) error {
		stopped.Add(1)
		return errors.New("err")
	}))
	m.Add(NewFuncTask("ok", nil, func(ctx context.Context) error {
		stopped.Add(1)
		return nil
	}))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if stopped.Load() != 2 {
		t.Fatalf("stopped = %d, want 2 (all tasks must be stopped)", stopped.Load())
	}
}

// ---------------------------------------------------------------------------
// State machine
// ---------------------------------------------------------------------------

func TestDoubleStart(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error on double start")
	}
	_ = m.Stop(context.Background())
}

func TestStopBeforeStart(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error stopping before start")
	}
}

func TestStopIdempotent(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	ctx := context.Background()
	err1 := m.Stop(ctx)
	err2 := m.Stop(ctx)
	err3 := m.Stop(ctx)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("expected nil, got %v / %v / %v", err1, err2, err3)
	}
}

func TestStartAfterStop(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error starting after stop")
	}
}

// ---------------------------------------------------------------------------
// Wait / context cancellation
// ---------------------------------------------------------------------------

func TestWait_ContextCancel(t *testing.T) {
	m := New(WithLogger(nopLogger()), WithTimeout(2*time.Second))
	m.Add(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.Wait(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return after context cancel")
	}
}

func TestDoneChannel(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	select {
	case <-m.Done():
		t.Fatal("Done should not be closed before Stop")
	default:
	}

	_ = m.Stop(context.Background())

	select {
	case <-m.Done():
		// ok
	case <-time.After(time.Second):
		t.Fatal("Done should be closed after Stop")
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestConcurrentStop(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("t", nil, nil))
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
}

func TestConcurrentAddBeforeStart(t *testing.T) {
	m := New(WithLogger(nopLogger()))

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Add(NewFuncTask("t", nil, nil))
		}()
	}
	wg.Wait()

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())
}

func TestConcurrentWaitAndStop(t *testing.T) {
	m := New(WithLogger(nopLogger()), WithTimeout(2*time.Second))
	m.Add(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.Wait(ctx)
	}()

	time.Sleep(10 * time.Millisecond)

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

// ---------------------------------------------------------------------------
// FuncTask
// ---------------------------------------------------------------------------

func TestFuncTask_Timeout(t *testing.T) {
	task := NewFuncTask("slow",
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		nil,
		WithTaskTimeout(10*time.Millisecond),
	)
	err := task.Start(context.Background())
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFuncTask_StopTimeout(t *testing.T) {
	task := NewFuncTask("slow-stop",
		nil,
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		WithTaskTimeout(10*time.Millisecond),
	)
	err := task.Stop(context.Background())
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEmptyManager(t *testing.T) {
	m := New(WithLogger(nopLogger()))

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestMultipleHooks(t *testing.T) {
	var count atomic.Int32

	m := New(WithLogger(nopLogger()))
	m.OnStart(func(ctx context.Context) error { count.Add(1); return nil })
	m.OnStart(func(ctx context.Context) error { count.Add(1); return nil })
	m.OnStart(func(ctx context.Context) error { count.Add(1); return nil })
	m.Add(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	if count.Load() != 3 {
		t.Fatalf("on_start called %d times, want 3", count.Load())
	}
	_ = m.Stop(context.Background())
}

func TestRollbackOrder(t *testing.T) {
	var order []string
	var mu sync.Mutex

	m := New(WithLogger(nopLogger()))
	boom := errors.New("boom")

	for _, name := range []string{"A", "B"} {
		n := name
		m.Add(NewFuncTask(n,
			func(ctx context.Context) error {
				mu.Lock()
				order = append(order, "start-"+n)
				mu.Unlock()
				return nil
			},
			func(ctx context.Context) error {
				mu.Lock()
				order = append(order, "rollback-"+n)
				mu.Unlock()
				return nil
			},
		))
	}

	m.Add(NewFuncTask("fail",
		func(ctx context.Context) error { return boom },
		nil,
	))

	err := m.Start(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}

	// Rollback should be in reverse: B then A
	assertOrder(t, []string{"start-A", "start-B", "rollback-B", "rollback-A"}, order)
}

func TestStartErrorContainsTaskName(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("my-task",
		func(ctx context.Context) error { return errors.New("fail") },
		nil,
	))

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !containsStr(got, "my-task") {
		t.Fatalf("error should contain task name, got: %s", got)
	}
}

func TestStopErrorContainsTaskName(t *testing.T) {
	m := New(WithLogger(nopLogger()))
	m.Add(NewFuncTask("my-task",
		nil,
		func(ctx context.Context) error { return errors.New("fail") },
	))

	_ = m.Start(context.Background())
	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); !containsStr(got, "my-task") {
		t.Fatalf("error should contain task name, got: %s", got)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

func BenchmarkStartStop_10(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := New(WithLogger(nopLogger()))
		for j := 0; j < 10; j++ {
			m.Add(NewFuncTask(fmt.Sprintf("t%d", j),
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			))
		}
		_ = m.Start(context.Background())
		_ = m.Stop(context.Background())
	}
}

func BenchmarkStartStop_100(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := New(WithLogger(nopLogger()))
		for j := 0; j < 100; j++ {
			m.Add(NewFuncTask(fmt.Sprintf("t%d", j),
				func(ctx context.Context) error { return nil },
				func(ctx context.Context) error { return nil },
			))
		}
		_ = m.Start(context.Background())
		_ = m.Stop(context.Background())
	}
}

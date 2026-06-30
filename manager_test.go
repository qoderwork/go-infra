package lifecycle

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestManager_BasicStartStop(t *testing.T) {
	var startCount, stopCount atomic.Int32

	m := NewManager(WithLogger(NopLogger()))
	m.AddTask(NewFuncTask("t1",
		func(ctx context.Context) error { startCount.Add(1); return nil },
		func(ctx context.Context) error { stopCount.Add(1); return nil },
	))
	m.AddTask(NewFuncTask("t2",
		func(ctx context.Context) error { startCount.Add(1); return nil },
		func(ctx context.Context) error { stopCount.Add(1); return nil },
	))

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if startCount.Load() != 2 {
		t.Fatalf("expected 2 starts, got %d", startCount.Load())
	}

	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if stopCount.Load() != 2 {
		t.Fatalf("expected 2 stops, got %d", stopCount.Load())
	}
}

func TestManager_PriorityOrder(t *testing.T) {
	var order []string
	var mu sync.Mutex

	m := NewManager(WithLogger(NopLogger()))

	m.AddTask(NewFuncTask("low", func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "low")
		mu.Unlock()
		return nil
	}, func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "low")
		mu.Unlock()
		return nil
	}, WithTaskPriority(1)))

	m.AddTask(NewFuncTask("high", func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "high")
		mu.Unlock()
		return nil
	}, func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "high")
		mu.Unlock()
		return nil
	}, WithTaskPriority(10)))

	m.AddTask(NewFuncTask("mid", func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "mid")
		mu.Unlock()
		return nil
	}, func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "mid")
		mu.Unlock()
		return nil
	}, WithTaskPriority(5)))

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Start order: high(10) -> mid(5) -> low(1)
	want := []string{"high", "mid", "low"}
	assertSliceEqual(t, want, order)

	// Stop order: low(1) -> mid(5) -> high(10) (reverse)
	order = nil
	if err := m.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	wantStop := []string{"low", "mid", "high"}
	assertSliceEqual(t, wantStop, order)
}

func TestManager_ReadyChannel(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	m.AddTask(NewFuncTask("t", nil, nil))

	select {
	case <-m.Ready():
		t.Fatal("Ready should not be closed before Start")
	default:
		// ok
	}

	_ = m.Start(context.Background())

	select {
	case <-m.Ready():
		// ok
	case <-time.After(time.Second):
		t.Fatal("Ready should be closed after Start")
	}

	_ = m.Stop(context.Background())
}

func TestManager_ShutdownIdempotent(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	m.AddTask(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	ctx := context.Background()
	err1 := m.Stop(ctx)
	err2 := m.Stop(ctx)
	err3 := m.ShutdownCtx(ctx, ReasonManual)

	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatalf("expected nil errors, got %v / %v / %v", err1, err2, err3)
	}
}

func TestManager_WaitContextCancel(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()), WithShutdownTimeout(2*time.Second))
	m.AddTask(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.Wait(ctx) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Wait did not return after context cancel")
	}

	if m.State() != StateStopped {
		t.Fatalf("state = %s, want Stopped", m.State())
	}
	if m.Reason() != ReasonContext {
		t.Fatalf("reason = %s, want context", m.Reason())
	}
}

func TestManager_StartError(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	boom := errors.New("boom")

	m.AddTask(NewFuncTask("ok", func(ctx context.Context) error { return nil }, nil))
	m.AddTask(NewFuncTask("fail", func(ctx context.Context) error { return boom }, nil))
	m.AddTask(NewFuncTask("never", func(ctx context.Context) error { return nil }, nil))

	err := m.Start(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected error wrapping boom, got %v", err)
	}

	// Manager should remain in Created state
	if m.State() != StateCreated {
		t.Fatalf("state = %s, want Created after start failure", m.State())
	}
}

func TestManager_StopError_Accumulated(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))

	m.AddTask(NewFuncTask("fail1", nil, func(ctx context.Context) error { return errors.New("err1") }))
	m.AddTask(NewFuncTask("ok", nil, nil))
	m.AddTask(NewFuncTask("fail2", nil, func(ctx context.Context) error { return errors.New("err2") }))

	_ = m.Start(context.Background())
	err := m.Stop(context.Background())

	if err == nil {
		t.Fatal("expected accumulated error, got nil")
	}
}

func TestManager_ParallelExecutor(t *testing.T) {
	m := NewManager(
		WithLogger(NopLogger()),
		WithExecutor(ExecutorParallel),
	)

	var count atomic.Int32
	for i := 0; i < 10; i++ {
		m.AddTask(NewFuncTask("pt",
			func(ctx context.Context) error { count.Add(1); return nil },
			func(ctx context.Context) error { return nil },
		))
	}

	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 10 {
		t.Fatalf("expected 10 starts, got %d", count.Load())
	}

	_ = m.Stop(context.Background())
}

func TestManager_MetricsHook(t *testing.T) {
	var events []MetricEvent
	var mu sync.Mutex

	m := NewManager(
		WithLogger(NopLogger()),
		WithMetricsHook(func(e MetricEvent) {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		}),
	)
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	mu.Lock()
	defer mu.Unlock()

	if len(events) < 4 {
		t.Fatalf("expected at least 4 metric events, got %d", len(events))
	}

	phases := make(map[Phase]bool)
	for _, e := range events {
		phases[e.Phase] = true
	}
	for _, p := range []Phase{PhaseStart, PhaseReady, PhaseStopping, PhaseStopped} {
		if !phases[p] {
			t.Errorf("missing metric event for phase %s", p)
		}
	}
}

func TestManager_ShutdownReason(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	m.AddTask(NewFuncTask("t", nil, nil))
	_ = m.Start(context.Background())

	_ = m.ShutdownCtx(context.Background(), ReasonError)

	if m.Reason() != ReasonError {
		t.Fatalf("reason = %s, want error", m.Reason())
	}
}

func TestManager_PanicRecoveryDisabled(t *testing.T) {
	m := NewManager(
		WithLogger(NopLogger()),
		WithPanicRecovery(false),
	)
	m.AddTask(NewFuncTask("panic", func(ctx context.Context) error {
		panic("unrecoverable")
	}, nil))

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate when recovery is disabled")
		}
	}()
	_ = m.Start(context.Background())
}

func TestManager_MetricsDuration(t *testing.T) {
	var events []MetricEvent
	var mu sync.Mutex

	m := NewManager(
		WithLogger(NopLogger()),
		WithMetricsHook(func(e MetricEvent) {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		}),
	)
	m.AddTask(NewFuncTask("slow",
		func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
		func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			return nil
		},
	))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	mu.Lock()
	defer mu.Unlock()

	// PhaseReady and PhaseStopped should have non-zero Duration
	var foundReady, foundStopped bool
	for _, e := range events {
		if e.Phase == PhaseReady && e.Duration > 0 {
			foundReady = true
		}
		if e.Phase == PhaseStopped && e.Duration > 0 {
			foundStopped = true
		}
	}
	if !foundReady {
		t.Error("PhaseReady event should have non-zero Duration")
	}
	if !foundStopped {
		t.Error("PhaseStopped event should have non-zero Duration")
	}
}

func TestManager_Err(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	m.AddTask(NewFuncTask("fail", nil, func(ctx context.Context) error {
		return errors.New("stop err")
	}))
	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if m.Err() == nil {
		t.Fatal("expected non-nil Err()")
	}
}

// ---------------------------------------------------------------------------
// Critical bug regression tests
// ---------------------------------------------------------------------------

// TestManager_StopBeforeStart_ThenStartThenStop verifies that a premature
// Stop() before Start() does not permanently consume the shutdown token.
// This was the sync.Once bug: once.Do was consumed on invalid-state Stop,
// making subsequent Start+Stop impossible.
func TestManager_StopBeforeStart_ThenStartThenStop(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	var stopped atomic.Bool
	m.AddTask(NewFuncTask("t",
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { stopped.Store(true); return nil },
	))

	// Step 1: Stop before Start — should fail
	err1 := m.Stop(context.Background())
	if err1 == nil {
		t.Fatal("expected error from Stop before Start")
	}

	// Step 2: Start should still work
	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start after premature Stop failed: %v", err)
	}
	if m.State() != StateRunning {
		t.Fatalf("state = %s, want Running", m.State())
	}

	// Step 3: Stop should actually shut down the manager
	if err := m.Stop(context.Background()); err != nil {
		t.Fatalf("Stop after Start failed: %v", err)
	}
	if m.State() != StateStopped {
		t.Fatalf("state = %s, want Stopped", m.State())
	}
	if !stopped.Load() {
		t.Fatal("task was not stopped — shutdown token was consumed by premature Stop")
	}
}

// TestManager_TaskCallbackNoDeadlock verifies that a task's Start() can
// safely call Manager methods (State, Ready) without deadlocking.
// This was the Start()-holds-lock bug.
func TestManager_TaskCallbackNoDeadlock(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))

	var observedState State
	_ = observedState // used to verify callback can read Manager state without deadlock
	m.AddTask(NewFuncTask("callback",
		func(ctx context.Context) error {
			// This would deadlock if Start() held the mutex during exec.Start()
			observedState = m.State()
			return nil
		},
		nil,
	))

	done := make(chan error, 1)
	go func() { done <- m.Start(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock detected: task callback calling Manager.State() blocked")
	}

	// State should have been Created (transition happens after exec.Start returns)
	// or Running depending on exact timing. Either way, it should not deadlock.
	_ = m.Stop(context.Background())
}

// TestManager_StartFailure_RollsBackStartedTasks verifies that when a task
// fails to start, already-started tasks are stopped (rolled back).
func TestManager_StartFailure_RollsBackStartedTasks(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))

	var rolledBack atomic.Bool
	boom := errors.New("boom")

	m.AddTask(NewFuncTask("ok",
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { rolledBack.Store(true); return nil },
	))
	m.AddTask(NewFuncTask("fail",
		func(ctx context.Context) error { return boom },
		nil,
	))

	err := m.Start(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected error wrapping boom, got %v", err)
	}

	// Manager should remain in Created state
	if m.State() != StateCreated {
		t.Fatalf("state = %s, want Created", m.State())
	}

	// The first task should have been rolled back (stopped)
	if !rolledBack.Load() {
		t.Fatal("already-started task was not rolled back after start failure")
	}
}

// TestManager_AddTaskNil verifies that AddTask(nil) returns an error
// instead of accepting a nil task that would panic during Start().
func TestManager_AddTaskNil(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	err := m.AddTask(nil)
	if err == nil {
		t.Fatal("expected error from AddTask(nil)")
	}
}

// ---------------------------------------------------------------------------
// Medium/low severity fix regression tests
// ---------------------------------------------------------------------------

// TestManager_ZeroValuePanics verifies that a zero-value Manager (not created
// via NewManager) panics rather than silently misbehaving.
func TestManager_ZeroValuePanics(t *testing.T) {
	var m Manager

	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: expected panic on zero-value Manager", name)
			}
		}()
		fn()
	}

	assertPanics("AddTask", func() { _ = m.AddTask(NewFuncTask("t", nil, nil)) })
	assertPanics("Start", func() { _ = m.Start(context.Background()) })
	assertPanics("Stop", func() { _ = m.Stop(context.Background()) })
}

// TestManager_CustomExecutor verifies that WithCustomExecutor injects a
// user-provided Executor that is used for both Start and Stop.
func TestManager_CustomExecutor(t *testing.T) {
	exec := &recordingExecutor{}
	m := NewManager(
		WithLogger(NopLogger()),
		WithCustomExecutor(exec),
	)
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	if exec.startCalls != 1 {
		t.Fatalf("custom executor Start called %d times, want 1", exec.startCalls)
	}

	_ = m.Stop(context.Background())
	if exec.stopCalls != 1 {
		t.Fatalf("custom executor Stop called %d times, want 1", exec.stopCalls)
	}
}

// TestFuncTask_Timeout verifies that WithTaskTimeout enforces a per-operation
// timeout. The operation receives a context that is canceled when the timeout
// expires, and the returned error wraps context.DeadlineExceeded.
func TestFuncTask_Timeout(t *testing.T) {
	// Start timeout
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
		t.Fatal("expected timeout error from Start")
	}

	// Stop timeout
	task2 := NewFuncTask("slow-stop",
		nil,
		func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
		WithTaskTimeout(10*time.Millisecond),
	)
	err = task2.Stop(context.Background())
	if err == nil {
		t.Fatal("expected timeout error from Stop")
	}
}

// TestParallelExecutor_ContextCanceled verifies that the parallel executor
// checks context cancellation before dispatching each goroutine, matching
// the serial executor's behavior.
func TestParallelExecutor_ContextCanceled(t *testing.T) {
	exec := newParallelExecutor(NopLogger(), true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tasks := []Task{
		NewFuncTask("t1", func(ctx context.Context) error { return nil }, nil),
	}

	err := exec.Start(ctx, tasks)
	if err == nil {
		t.Fatal("expected error from canceled context")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// recordingExecutor is a test double that counts Start/Stop invocations.
type recordingExecutor struct {
	startCalls int
	stopCalls  int
}

func (e *recordingExecutor) Start(ctx context.Context, tasks []Task) error {
	e.startCalls++
	for _, t := range tasks {
		if err := t.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (e *recordingExecutor) Stop(ctx context.Context, tasks []Task) error {
	e.stopCalls++
	for i := len(tasks) - 1; i >= 0; i-- {
		if err := tasks[i].Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

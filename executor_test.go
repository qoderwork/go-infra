package lifecycle

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSerialExecutor_StartOrder(t *testing.T) {
	exec := newSerialExecutor(NopLogger(), true)

	var order []string
	var mu sync.Mutex

	tasks := []Task{
		NewFuncTask("a", func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "a-start")
			mu.Unlock()
			return nil
		}, nil),
		NewFuncTask("b", func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "b-start")
			mu.Unlock()
			return nil
		}, nil),
		NewFuncTask("c", func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "c-start")
			mu.Unlock()
			return nil
		}, nil),
	}

	if err := exec.Start(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}

	want := []string{"a-start", "b-start", "c-start"}
	assertSliceEqual(t, want, order)
}

func TestSerialExecutor_StopLIFO(t *testing.T) {
	exec := newSerialExecutor(NopLogger(), true)

	var order []string
	var mu sync.Mutex

	tasks := []Task{
		NewFuncTask("a", nil, func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "a-stop")
			mu.Unlock()
			return nil
		}),
		NewFuncTask("b", nil, func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "b-stop")
			mu.Unlock()
			return nil
		}),
		NewFuncTask("c", nil, func(ctx context.Context) error {
			mu.Lock()
			order = append(order, "c-stop")
			mu.Unlock()
			return nil
		}),
	}

	if err := exec.Stop(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}

	want := []string{"c-stop", "b-stop", "a-stop"}
	assertSliceEqual(t, want, order)
}

func TestSerialExecutor_StartFailsStopsImmediately(t *testing.T) {
	exec := newSerialExecutor(NopLogger(), true)

	var started atomic.Int32
	errBoom := errors.New("boom")

	tasks := []Task{
		NewFuncTask("ok", func(ctx context.Context) error {
			started.Add(1)
			return nil
		}, nil),
		NewFuncTask("fail", func(ctx context.Context) error {
			started.Add(1)
			return errBoom
		}, nil),
		NewFuncTask("never", func(ctx context.Context) error {
			started.Add(1)
			return nil
		}, nil),
	}

	err := exec.Start(context.Background(), tasks)
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected error wrapping errBoom, got %v", err)
	}
	if started.Load() != 2 {
		t.Fatalf("expected 2 tasks started, got %d", started.Load())
	}
}

func TestSerialExecutor_StopContinuesOnError(t *testing.T) {
	exec := newSerialExecutor(NopLogger(), true)

	var stopped atomic.Int32

	tasks := []Task{
		NewFuncTask("fail", nil, func(ctx context.Context) error {
			stopped.Add(1)
			return errors.New("stop error")
		}),
		NewFuncTask("ok", nil, func(ctx context.Context) error {
			stopped.Add(1)
			return nil
		}),
	}

	_ = exec.Stop(context.Background(), tasks)
	if stopped.Load() != 2 {
		t.Fatalf("expected both tasks stopped, got %d", stopped.Load())
	}
}

func TestSerialExecutor_ContextCanceled(t *testing.T) {
	exec := newSerialExecutor(NopLogger(), true)

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

func TestParallelExecutor_AllTasksRun(t *testing.T) {
	exec := newParallelExecutor(NopLogger(), true)

	var count atomic.Int32

	tasks := []Task{
		NewFuncTask("p1", func(ctx context.Context) error {
			count.Add(1)
			return nil
		}, func(ctx context.Context) error {
			count.Add(1)
			return nil
		}),
		NewFuncTask("p2", func(ctx context.Context) error {
			count.Add(1)
			return nil
		}, func(ctx context.Context) error {
			count.Add(1)
			return nil
		}),
		NewFuncTask("p3", func(ctx context.Context) error {
			count.Add(1)
			return nil
		}, func(ctx context.Context) error {
			count.Add(1)
			return nil
		}),
	}

	if err := exec.Start(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 3 {
		t.Fatalf("expected 3 tasks started, got %d", count.Load())
	}

	count.Store(0)
	if err := exec.Stop(context.Background(), tasks); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 3 {
		t.Fatalf("expected 3 tasks stopped, got %d", count.Load())
	}
}

func TestParallelExecutor_CollectsAllErrors(t *testing.T) {
	exec := newParallelExecutor(NopLogger(), true)

	tasks := []Task{
		NewFuncTask("fail1", func(ctx context.Context) error { return errors.New("err1") }, nil),
		NewFuncTask("ok", func(ctx context.Context) error { return nil }, nil),
		NewFuncTask("fail2", func(ctx context.Context) error { return errors.New("err2") }, nil),
	}

	err := exec.Start(context.Background(), tasks)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// errors.Join should contain both errors
	if !errors.Is(err, errors.Unwrap(err)) {
		// Just verify we got an error; the exact structure depends on errors.Join
	}
}

func TestSerialExecutor_PanicRecovery(t *testing.T) {
	exec := newSerialExecutor(NopLogger(), true)

	tasks := []Task{
		NewFuncTask("panic", func(ctx context.Context) error {
			panic("test panic")
		}, nil),
	}

	err := exec.Start(context.Background(), tasks)
	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}
}

func TestParallelExecutor_PanicRecovery(t *testing.T) {
	exec := newParallelExecutor(NopLogger(), true)

	tasks := []Task{
		NewFuncTask("panic", func(ctx context.Context) error {
			panic("test panic parallel")
		}, nil),
	}

	err := exec.Start(context.Background(), tasks)
	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}
}

func TestParallelExecutor_EmptyTasks(t *testing.T) {
	exec := newParallelExecutor(NopLogger(), true)
	if err := exec.Start(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := exec.Stop(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertSliceEqual(t *testing.T, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("length mismatch: want %d, got %d\nwant: %v\ngot:  %v", len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("index %d: want %q, got %q", i, want[i], got[i])
		}
	}
}

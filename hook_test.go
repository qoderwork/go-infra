package lifecycle

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestHooks_OnStart(t *testing.T) {
	var called atomic.Int32

	m := NewManager(WithLogger(NopLogger()))
	m.OnStart(func(ctx context.Context) error {
		called.Add(1)
		return nil
	})
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	if called.Load() != 1 {
		t.Fatalf("OnStart called %d times, want 1", called.Load())
	}
	_ = m.Stop(context.Background())
}

func TestHooks_OnStartFails_AbortsStart(t *testing.T) {
	boom := errors.New("hook boom")
	m := NewManager(WithLogger(NopLogger()))
	m.OnStart(func(ctx context.Context) error { return boom })
	m.AddTask(NewFuncTask("t", func(ctx context.Context) error {
		t.Fatal("task should not have been started")
		return nil
	}, nil))

	err := m.Start(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("expected error wrapping boom, got %v", err)
	}
}

func TestHooks_OnReady(t *testing.T) {
	var called atomic.Int32

	m := NewManager(WithLogger(NopLogger()))
	m.OnReady(func(ctx context.Context) error {
		called.Add(1)
		return nil
	})
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	if called.Load() != 1 {
		t.Fatalf("OnReady called %d times, want 1", called.Load())
	}
	_ = m.Stop(context.Background())
}

func TestHooks_OnReadyError_NonFatal(t *testing.T) {
	m := NewManager(WithLogger(NopLogger()))
	m.OnReady(func(ctx context.Context) error { return errors.New("ready err") })
	m.AddTask(NewFuncTask("t", nil, nil))

	err := m.Start(context.Background())
	if err != nil {
		t.Fatalf("OnReady error should not prevent Start, got %v", err)
	}
	if m.State() != StateRunning {
		t.Fatalf("state = %s, want Running", m.State())
	}
	_ = m.Stop(context.Background())
}

func TestHooks_OnStopping(t *testing.T) {
	var called atomic.Int32

	m := NewManager(WithLogger(NopLogger()))
	m.OnStopping(func(ctx context.Context) error {
		called.Add(1)
		return nil
	})
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if called.Load() != 1 {
		t.Fatalf("OnStopping called %d times, want 1", called.Load())
	}
}

func TestHooks_OnStopped(t *testing.T) {
	var called atomic.Int32

	m := NewManager(WithLogger(NopLogger()))
	m.OnStopped(func(ctx context.Context) error {
		called.Add(1)
		return nil
	})
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	if called.Load() != 1 {
		t.Fatalf("OnStopped called %d times, want 1", called.Load())
	}
}

func TestHooks_AllFourPhases(t *testing.T) {
	var order []string
	m := NewManager(WithLogger(NopLogger()))

	m.OnStart(func(ctx context.Context) error {
		order = append(order, "on_start")
		return nil
	})
	m.OnReady(func(ctx context.Context) error {
		order = append(order, "on_ready")
		return nil
	})
	m.OnStopping(func(ctx context.Context) error {
		order = append(order, "on_stopping")
		return nil
	})
	m.OnStopped(func(ctx context.Context) error {
		order = append(order, "on_stopped")
		return nil
	})

	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	_ = m.Stop(context.Background())

	want := []string{"on_start", "on_ready", "on_stopping", "on_stopped"}
	assertSliceEqual(t, want, order)
}

func TestHooks_MultipleHooks(t *testing.T) {
	var count atomic.Int32

	m := NewManager(WithLogger(NopLogger()))
	m.OnStart(func(ctx context.Context) error { count.Add(1); return nil })
	m.OnStart(func(ctx context.Context) error { count.Add(1); return nil })
	m.OnStart(func(ctx context.Context) error { count.Add(1); return nil })
	m.AddTask(NewFuncTask("t", nil, nil))

	_ = m.Start(context.Background())
	if count.Load() != 3 {
		t.Fatalf("expected 3 OnStart hooks called, got %d", count.Load())
	}
	_ = m.Stop(context.Background())
}

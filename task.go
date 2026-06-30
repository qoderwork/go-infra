package lifecycle

import (
	"context"
	"fmt"
	"time"
)

// Task represents a managed unit of work with a lifecycle.
//
// Each task has a name for identification and logging, a priority for
// execution ordering, and Start/Stop methods for lifecycle management.
//
// Start is called when the Manager starts. Stop is called when the
// Manager shuts down. Both methods receive a context that may carry
// a deadline; implementations should respect context cancellation.
type Task interface {
	// Name returns the human-readable name of the task.
	// It is used in logging, metrics, and error messages.
	Name() string

	// Priority returns the task's execution priority.
	// Higher values start first and stop last (analogous to defer order).
	// Tasks with equal priority maintain their registration order.
	Priority() int

	// Start initializes and begins the task.
	// It should return nil once the task is fully operational.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the task.
	// It should block until all resources are released or ctx expires.
	Stop(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// FuncTask — convenience adapter
// ---------------------------------------------------------------------------

// FuncTaskOption configures a FuncTask.
type FuncTaskOption func(*FuncTask)

// FuncTask implements Task using plain functions.
//
// It is the recommended way to create tasks for components that do not
// need a dedicated type:
//
//	task := lifecycle.NewFuncTask("redis",
//	    func(ctx context.Context) error { return redis.Connect() },
//	    func(ctx context.Context) error { return redis.Close() },
//	    lifecycle.WithTaskPriority(10),
//	)
type FuncTask struct {
	name     string
	priority int
	timeout  time.Duration
	startFn  func(context.Context) error
	stopFn   func(context.Context) error
}

// NewFuncTask creates a FuncTask with the given name, start and stop functions.
// Either start or stop may be nil; the corresponding operation becomes a no-op.
func NewFuncTask(name string, start, stop func(context.Context) error, opts ...FuncTaskOption) *FuncTask {
	t := &FuncTask{
		name:    name,
		startFn: start,
		stopFn:  stop,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// WithTaskPriority sets the execution priority of a FuncTask.
func WithTaskPriority(p int) FuncTaskOption {
	return func(t *FuncTask) { t.priority = p }
}

// WithTaskTimeout sets a per-operation timeout for a FuncTask.
// When set, each Start and Stop call receives a context with a deadline.
// If the operation exceeds the timeout, the context is canceled and
// context.DeadlineExceeded is returned.
//
//	task := lifecycle.NewFuncTask("slow",
//	    slowStart, slowStop,
//	    lifecycle.WithTaskTimeout(5*time.Second),
//	)
func WithTaskTimeout(d time.Duration) FuncTaskOption {
	return func(t *FuncTask) { t.timeout = d }
}

func (f *FuncTask) Name() string  { return f.name }
func (f *FuncTask) Priority() int { return f.priority }

func (f *FuncTask) Start(ctx context.Context) error {
	if f.startFn == nil {
		return nil
	}
	if f.timeout > 0 {
		tctx, cancel := context.WithTimeout(ctx, f.timeout)
		defer cancel()
		err := f.startFn(tctx)
		if tctx.Err() != nil {
			return fmt.Errorf("lifecycle: task %q start timed out: %w", f.name, tctx.Err())
		}
		return err
	}
	return f.startFn(ctx)
}

func (f *FuncTask) Stop(ctx context.Context) error {
	if f.stopFn == nil {
		return nil
	}
	if f.timeout > 0 {
		tctx, cancel := context.WithTimeout(ctx, f.timeout)
		defer cancel()
		err := f.stopFn(tctx)
		if tctx.Err() != nil {
			return fmt.Errorf("lifecycle: task %q stop timed out: %w", f.name, tctx.Err())
		}
		return err
	}
	return f.stopFn(ctx)
}

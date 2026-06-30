package lifecycle

import (
	"context"
	"fmt"
)

// Executor defines the strategy for starting and stopping a batch of tasks.
//
// Two implementations are provided:
//   - serialExecutor: tasks are processed one-by-one (default).
//   - parallelExecutor: tasks are processed concurrently.
type Executor interface {
	// Start starts the given tasks in order.
	// If a task fails to start, the executor decides whether to continue
	// or abort (serial stops at first error; parallel collects all errors).
	Start(ctx context.Context, tasks []Task) error

	// Stop stops the given tasks.
	// Implementations should attempt to stop all tasks even if some fail.
	Stop(ctx context.Context, tasks []Task) error
}

// ---------------------------------------------------------------------------
// Panic recovery helper (shared by both executors)
// ---------------------------------------------------------------------------

// safeExec runs fn, optionally recovering from panics.
// When enableRecovery is true, any panic is caught and converted to an error.
// When enableRecovery is false, panics propagate normally.
func safeExec(logger Logger, enableRecovery bool, fn func() error) (retErr error) {
	if enableRecovery {
		defer func() {
			if r := recover(); r != nil {
				if logger != nil {
					logger.Printf("lifecycle: recovered panic: %v", r)
				}
				retErr = fmt.Errorf("lifecycle: panic recovered: %v", r)
			}
		}()
	}
	return fn()
}

// ---------------------------------------------------------------------------
// serialExecutor
// ---------------------------------------------------------------------------

// serialExecutor processes tasks one at a time.
// Start iterates in registration order; Stop iterates in reverse (LIFO).
type serialExecutor struct {
	logger        Logger
	panicRecovery bool
}

func newSerialExecutor(logger Logger, panicRecovery bool) *serialExecutor {
	return &serialExecutor{logger: logger, panicRecovery: panicRecovery}
}

func (e *serialExecutor) Start(ctx context.Context, tasks []Task) error {
	for _, t := range tasks {
		select {
		case <-ctx.Done():
			return fmt.Errorf("lifecycle: context canceled before starting task %q: %w", t.Name(), ctx.Err())
		default:
		}

		if err := safeExec(e.logger, e.panicRecovery, func() error { return t.Start(ctx) }); err != nil {
			return fmt.Errorf("lifecycle: failed to start task %q: %w", t.Name(), err)
		}
	}
	return nil
}

func (e *serialExecutor) Stop(ctx context.Context, tasks []Task) error {
	var errs []error
	for i := len(tasks) - 1; i >= 0; i-- {
		t := tasks[i]
		if err := safeExec(e.logger, e.panicRecovery, func() error { return t.Stop(ctx) }); err != nil {
			errs = append(errs, fmt.Errorf("lifecycle: failed to stop task %q: %w", t.Name(), err))
		}
	}
	return joinErrors(errs)
}

// ---------------------------------------------------------------------------
// parallelExecutor
// ---------------------------------------------------------------------------

// parallelExecutor processes all tasks concurrently.
type parallelExecutor struct {
	logger        Logger
	panicRecovery bool
}

func newParallelExecutor(logger Logger, panicRecovery bool) *parallelExecutor {
	return &parallelExecutor{logger: logger, panicRecovery: panicRecovery}
}

func (e *parallelExecutor) Start(ctx context.Context, tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}

	type result struct {
		name string
		err  error
	}

	ch := make(chan result, len(tasks))
	for _, t := range tasks {
		// Check context before dispatching each goroutine, matching
		// the serial executor's per-task context check.
		select {
		case <-ctx.Done():
			return fmt.Errorf("lifecycle: context canceled before starting task %q: %w", t.Name(), ctx.Err())
		default:
		}

		go func(task Task) {
			err := safeExec(e.logger, e.panicRecovery, func() error { return task.Start(ctx) })
			ch <- result{name: task.Name(), err: err}
		}(t)
	}

	var errs []error
	for range tasks {
		r := <-ch
		if r.err != nil {
			errs = append(errs, fmt.Errorf("lifecycle: failed to start task %q: %w", r.name, r.err))
		}
	}
	return joinErrors(errs)
}

func (e *parallelExecutor) Stop(ctx context.Context, tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}

	type result struct {
		name string
		err  error
	}

	ch := make(chan result, len(tasks))
	for _, t := range tasks {
		go func(task Task) {
			err := safeExec(e.logger, e.panicRecovery, func() error { return task.Stop(ctx) })
			ch <- result{name: task.Name(), err: err}
		}(t)
	}

	var errs []error
	for range tasks {
		r := <-ch
		if r.err != nil {
			errs = append(errs, fmt.Errorf("lifecycle: failed to stop task %q: %w", r.name, r.err))
		}
	}
	return joinErrors(errs)
}

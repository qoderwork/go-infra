// Package lifecycle provides a minimal application lifecycle manager.
//
// It coordinates the startup and graceful shutdown of multiple tasks
// using a simple model: tasks are started in registration order and
// stopped in reverse order (LIFO, like defer).
//
// # Quick Start
//
//	mgr := lifecycle.New()
//	mgr.Add(myDB)
//	mgr.Add(myHTTPServer)
//	mgr.Run() // blocks until SIGINT/SIGTERM, then gracefully stops
//
// # Design
//
// Registration order IS the priority. The first task added starts first
// and stops last. No priority numbers, no dependency graphs.
//
// The Manager starts all tasks synchronously in the caller's goroutine.
// No hidden goroutines (except Go runtime's signal handler).
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Task
// ---------------------------------------------------------------------------

// Task is a managed unit of work with a lifecycle.
type Task interface {
	// Name returns a human-readable identifier for logging and errors.
	Name() string

	// Start initializes the task. Called in registration order.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the task. Called in reverse order (LIFO).
	Stop(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// Manager
// ---------------------------------------------------------------------------

// Manager coordinates the lifecycle of a set of Tasks.
//
// Tasks are started in registration order and stopped in reverse order.
// All methods are safe for concurrent use.
type Manager struct {
	mu      sync.Mutex
	running bool
	stopped bool
	tasks   []Task

	startTimeout time.Duration
	stopTimeout  time.Duration
	logger       *slog.Logger

	onStartHooks []func(ctx context.Context) error
	onStopHooks  []func(ctx context.Context) error

	stopOnce  sync.Once
	closeOnce sync.Once
	stopErr   error
	done      chan struct{}
}

// New creates a Manager with the given options.
//
// Defaults: startTimeout 30s, stopTimeout 30s, logger slog.Default().
func New(opts ...Option) *Manager {
	m := &Manager{
		startTimeout: 30 * time.Second,
		stopTimeout:  30 * time.Second,
		logger:       slog.Default(),
		done:         make(chan struct{}),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Add registers a task. Must be called before Start.
// Nil tasks are silently ignored. After Start or Stop, Add logs a warning
// and returns without registering the task.
func (m *Manager) Add(t Task) {
	if t == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running || m.stopped {
		m.logger.Warn("lifecycle: Add called after Start/Stop, ignoring", "task", t.Name())
		return
	}
	m.tasks = append(m.tasks, t)
}

// OnStart registers a hook called before any task is started.
// If any hook returns an error, Start aborts without starting any task.
func (m *Manager) OnStart(fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStartHooks = append(m.onStartHooks, fn)
}

// OnStop registers a hook called after all tasks have been stopped.
// Errors are accumulated into the Stop return value.
func (m *Manager) OnStop(fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStopHooks = append(m.onStopHooks, fn)
}

// ---------------------------------------------------------------------------
// Lifecycle control
// ---------------------------------------------------------------------------

// Start transitions the Manager to running and starts all tasks
// in registration order.
//
// If any task fails to start, already-started tasks are rolled back
// (stopped in reverse order) and the error is returned.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return errors.New("lifecycle: already running")
	}
	if m.stopped {
		m.mu.Unlock()
		return errors.New("lifecycle: cannot restart after stop")
	}
	tasks := make([]Task, len(m.tasks))
	copy(tasks, m.tasks)
	hooks := make([]func(context.Context) error, len(m.onStartHooks))
	copy(hooks, m.onStartHooks)
	m.mu.Unlock()

	// Execute OnStart hooks
	for _, fn := range hooks {
		if err := fn(ctx); err != nil {
			m.mu.Lock()
			m.stopped = true
			m.mu.Unlock()
			m.closeDone()
			return fmt.Errorf("lifecycle: on_start: %w", err)
		}
	}

	// Start tasks in order; rollback on failure
	var started []Task
	for _, t := range tasks {
		if err := t.Start(ctx); err != nil {
			m.rollback(started)
			m.mu.Lock()
			m.stopped = true
			m.mu.Unlock()
			m.closeDone()
			return fmt.Errorf("lifecycle: start %q: %w", t.Name(), err)
		}
		started = append(started, t)
	}

	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	m.logger.Info("lifecycle: started", "tasks", len(tasks))
	return nil
}

// Stop shuts down all tasks in reverse order (LIFO).
// Safe to call multiple times; only the first call has effect.
func (m *Manager) Stop(ctx context.Context) error {
	m.stopOnce.Do(func() {
		m.mu.Lock()
		if !m.running {
			m.stopped = true
			m.stopErr = errors.New("lifecycle: not running")
			m.mu.Unlock()
			m.closeDone()
			return
		}
		tasks := make([]Task, len(m.tasks))
		copy(tasks, m.tasks)
		hooks := make([]func(context.Context) error, len(m.onStopHooks))
		copy(hooks, m.onStopHooks)
		m.running = false
		m.stopped = true
		m.mu.Unlock()

		defer m.closeDone()

		// Stop in reverse order with panic protection
		var errs []error
		for i := len(tasks) - 1; i >= 0; i-- {
			func(t Task) {
				defer func() {
					if r := recover(); r != nil {
						m.logger.Error("lifecycle: stop panicked", "task", t.Name(), "panic", r)
						errs = append(errs, fmt.Errorf("lifecycle: stop %q: panic: %v", t.Name(), r))
					}
				}()
				if err := t.Stop(ctx); err != nil {
					errs = append(errs, fmt.Errorf("lifecycle: stop %q: %w", t.Name(), err))
				}
			}(tasks[i])
		}

		// Execute OnStop hooks
		for _, fn := range hooks {
			func(fn func(context.Context) error) {
				defer func() {
					if r := recover(); r != nil {
						m.logger.Error("lifecycle: on_stop panicked", "panic", r)
						errs = append(errs, fmt.Errorf("lifecycle: on_stop: panic: %v", r))
					}
				}()
				if err := fn(ctx); err != nil {
					errs = append(errs, fmt.Errorf("lifecycle: on_stop: %w", err))
				}
			}(fn)
		}

		m.mu.Lock()
		m.stopErr = errors.Join(errs...)
		m.mu.Unlock()

		m.logger.Info("lifecycle: stopped", "tasks", len(tasks))
	})
	return m.stopErr
}

// Wait blocks until ctx is done, then initiates shutdown.
func (m *Manager) Wait(ctx context.Context) error {
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), m.stopTimeout)
	defer cancel()
	return m.Stop(shutdownCtx)
}

// Run starts all tasks, waits for an OS signal, then gracefully stops.
// This is the recommended entry point for most applications.
// Startup is bounded by startTimeout; shutdown by stopTimeout.
func (m *Manager) Run() error {
	startCtx, cancel := context.WithTimeout(context.Background(), m.startTimeout)
	defer cancel()
	if err := m.Start(startCtx); err != nil {
		return err
	}
	if err := m.WaitSignal(m.stopTimeout); err != nil {
		return err
	}
	return nil
}

// Done returns a channel that is closed when the Manager has fully stopped.
func (m *Manager) Done() <-chan struct{} {
	return m.done
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

func (m *Manager) closeDone() {
	m.closeOnce.Do(func() { close(m.done) })
}

func (m *Manager) rollback(started []Task) {
	ctx, cancel := context.WithTimeout(context.Background(), m.stopTimeout)
	defer cancel()
	for i := len(started) - 1; i >= 0; i-- {
		func(t Task) {
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error("lifecycle: rollback panicked", "task", t.Name(), "panic", r)
				}
			}()
			if err := t.Stop(ctx); err != nil {
				m.logger.Error("lifecycle: rollback failed", "task", t.Name(), "err", err)
			}
		}(started[i])
	}
}

// Package lifecycle provides a production-grade application lifecycle manager
// for Go. It coordinates the startup and graceful shutdown of multiple
// concurrent tasks (HTTP servers, database connections, message consumers,
// background workers, etc.) with configurable execution strategies, hook
// points, metrics integration, and panic recovery.
//
// Inspired by oklog/run and Uber Fx Lifecycle, but deliberately lighter:
// no dependency injection framework, no code generation, just clean Go.
//
// # Quick Start
//
//	mgr := lifecycle.NewManager(
//	    lifecycle.WithLogger(slog.Default()),
//	    lifecycle.WithShutdownTimeout(15 * time.Second),
//	)
//	mgr.AddTask(myHTTPServer)
//	mgr.AddTask(myDBConnection)
//	if err := mgr.Run(); err != nil {
//	    log.Fatal(err)
//	}
//
// # State Machine
//
// The Manager follows a strict state progression:
//
//	Created -> Running -> Stopping -> Stopped
//
// # Execution Order
//
// Tasks are sorted by Priority (descending) before execution.
// Higher-priority tasks start first and stop last, similar to defer.
// Within the same priority level, registration order is preserved.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager orchestrates the lifecycle of a set of Tasks.
//
// A Manager is safe for concurrent use. All exported methods use internal
// synchronization to protect shared state.
type Manager struct {
	mu    sync.Mutex
	state State
	tasks []Task

	// Configuration
	executorType    ExecutorType
	customExecutor  Executor
	logger          Logger
	metricsHook     MetricsHook
	shutdownTimeout time.Duration
	panicRecovery   bool

	// Hooks
	onStartHooks    []Hook
	onReadyHooks    []Hook
	onStoppingHooks []Hook
	onStoppedHooks  []Hook

	// Shutdown coordination
	//
	// shutdownInitiated is set under mu to ensure only one goroutine
	// performs the actual shutdown work. Unlike sync.Once, it is reset
	// by Start() so that a premature Stop() before Start() does not
	// permanently consume the shutdown token.
	//
	// stopDone is closed when shutdown completes, allowing concurrent
	// callers to block until the Manager reaches Stopped state.
	shutdownInitiated bool
	stopDone          chan struct{}
	reason            ShutdownReason
	shutErr           error
	readyCh           chan struct{}
	closeReady        sync.Once
}

// NewManager creates a new lifecycle Manager with the given options.
//
// Defaults:
//   - Executor: serial (LIFO stop order)
//   - Logger:   slog.Default()
//   - Shutdown timeout: 30s
//   - Panic recovery: enabled
func NewManager(opts ...Option) *Manager {
	m := &Manager{
		state:           StateCreated,
		logger:          defaultLogger(),
		metricsHook:     func(MetricEvent) {},
		shutdownTimeout: 30 * time.Second,
		panicRecovery:   true,
		readyCh:         make(chan struct{}),
		stopDone:        make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// ---------------------------------------------------------------------------
// Task registration
// ---------------------------------------------------------------------------

// AddTask registers a task with the Manager.
//
// Tasks can only be added in the Created state. Calling AddTask after
// Start returns an error.
//
// AddTask panics if the Manager was not created via NewManager.
func (m *Manager) AddTask(t Task) error {
	if t == nil {
		return errors.New("lifecycle: cannot add nil task")
	}
	if !m.initialized() {
		panic("lifecycle: use of uninitialized Manager; call NewManager")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateCreated {
		return fmt.Errorf("lifecycle: cannot add task in state %s", m.state)
	}
	m.tasks = append(m.tasks, t)
	return nil
}

// ---------------------------------------------------------------------------
// Lifecycle control
// ---------------------------------------------------------------------------

// Start transitions the Manager from Created to Running and starts all
// registered tasks according to the configured executor strategy.
//
// Tasks are sorted by Priority (descending) before starting. If any task
// fails to start, already-started tasks are rolled back (stopped in reverse
// order) and the error is returned. The Manager remains in Created state so
// the caller can retry or inspect the failure.
func (m *Manager) Start(ctx context.Context) error {
	if !m.initialized() {
		panic("lifecycle: use of uninitialized Manager; call NewManager")
	}

	// Phase 1: Validate state and snapshot under lock.
	m.mu.Lock()
	if m.state != StateCreated {
		m.mu.Unlock()
		return fmt.Errorf("lifecycle: cannot start in state %s", m.state)
	}

	// Reset shutdown state so a prior Stop-before-Start does not prevent
	// future shutdown.
	m.shutdownInitiated = false
	m.shutErr = nil
	m.reason = ""
	m.stopDone = make(chan struct{})

	// Snapshot tasks and hooks, then release lock for long-running work.
	tasksCopy := make([]Task, len(m.tasks))
	copy(tasksCopy, m.tasks)
	startHooks := make([]Hook, len(m.onStartHooks))
	copy(startHooks, m.onStartHooks)
	readyHooks := make([]Hook, len(m.onReadyHooks))
	copy(readyHooks, m.onReadyHooks)
	m.mu.Unlock()

	// Phase 2: Execute outside lock — tasks may call back into Manager.
	m.logger.Printf("lifecycle: starting manager with %d task(s)", len(tasksCopy))
	startAt := time.Now()
	m.emit(PhaseStart, "manager", 0, nil)

	// Execute OnStart hooks
	for i, h := range startHooks {
		if err := h(ctx); err != nil {
			m.logger.Printf("lifecycle: on_start hook[%d] failed: %v", i, err)
			m.emit(PhaseStart, "manager", time.Since(startAt), err)
			return fmt.Errorf("lifecycle: on_start hook failed: %w", err)
		}
	}

	// Sort tasks by priority (descending) — stable sort preserves registration
	// order within the same priority level.
	sorted := make([]Task, len(tasksCopy))
	copy(sorted, tasksCopy)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})

	// Build executor and start all tasks
	exec := m.buildExecutor()
	startErr := exec.Start(ctx, sorted)

	if startErr != nil {
		m.logger.Printf("lifecycle: start failed: %v", startErr)
		m.emit(PhaseStart, "manager", time.Since(startAt), startErr)

		// Rollback: stop already-started tasks in reverse order to prevent
		// resource leaks. Uses a fresh context so rollback is not blocked
		// by the caller's potentially-canceled context.
		m.logger.Printf("lifecycle: rolling back %d already-started task(s)", len(sorted))
		rollbackCtx := context.Background()
		for i := len(sorted) - 1; i >= 0; i-- {
			if err := sorted[i].Stop(rollbackCtx); err != nil {
				m.logger.Printf("lifecycle: rollback stop of task %q failed: %v", sorted[i].Name(), err)
			}
		}
		return startErr
	}

	// Phase 3: Transition to Running under lock.
	m.mu.Lock()
	m.state = StateRunning
	m.logger.Printf("lifecycle: manager is now Running")
	m.mu.Unlock()

	m.emit(PhaseReady, "manager", time.Since(startAt), nil)

	// Execute OnReady hooks (outside lock, errors logged but non-fatal)
	for i, h := range readyHooks {
		if err := h(ctx); err != nil {
			m.logger.Printf("lifecycle: on_ready hook[%d] failed: %v", i, err)
		}
	}

	// Signal readiness
	m.closeReady.Do(func() { close(m.readyCh) })

	return nil
}

// Stop transitions the Manager from Running to Stopped, shutting down all
// tasks according to the configured executor strategy.
//
// Stop is safe to call multiple times; only the first call has effect.
func (m *Manager) Stop(ctx context.Context) error {
	return m.doStop(ctx, ReasonManual)
}

// ShutdownCtx initiates shutdown with a specific reason and context.
func (m *Manager) ShutdownCtx(ctx context.Context, reason ShutdownReason) error {
	return m.doStop(ctx, reason)
}

// doStop is the internal implementation of all shutdown paths.
//
// Shutdown coordination uses two mechanisms:
//   - shutdownInitiated (under mu): ensures only one goroutine performs the
//     actual shutdown work. Set atomically with the state check.
//   - stopDone (channel): closed when shutdown completes, so concurrent
//     callers can block until the Manager reaches Stopped state.
//
// Unlike sync.Once, shutdownInitiated is reset by Start(), so a premature
// Stop() before Start() returns an error without permanently consuming the
// shutdown token.
func (m *Manager) doStop(ctx context.Context, reason ShutdownReason) error {
	if !m.initialized() {
		panic("lifecycle: use of uninitialized Manager; call NewManager")
	}

	// Phase 1: Try to claim the shutdown role.
	m.mu.Lock()

	// If another goroutine already initiated shutdown, wait for completion.
	if m.shutdownInitiated {
		m.mu.Unlock()
		<-m.stopDone
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.shutErr
	}

	// Validate state BEFORE claiming shutdown. If not Running, return error
	// without consuming the shutdown token — Start() can still succeed later.
	if m.state != StateRunning {
		err := fmt.Errorf("lifecycle: cannot stop in state %s", m.state)
		m.shutErr = err
		m.mu.Unlock()
		return err
	}

	// Claim shutdown role.
	m.shutdownInitiated = true
	m.state = StateStopping
	m.reason = reason
	m.logger.Printf("lifecycle: stopping manager (reason: %s)", reason)
	stopAt := time.Now()
	m.emit(PhaseStopping, "manager", 0, nil)

	// Snapshot tasks and hooks, then release lock for long-running work.
	sorted := make([]Task, len(m.tasks))
	copy(sorted, m.tasks)
	stoppingHooks := make([]Hook, len(m.onStoppingHooks))
	copy(stoppingHooks, m.onStoppingHooks)
	m.mu.Unlock()

	// Phase 2: Execute shutdown outside lock.
	// Ensure stopDone is closed exactly once when we finish.
	defer close(m.stopDone)

	// Execute OnStopping hooks
	for i, h := range stoppingHooks {
		if err := h(ctx); err != nil {
			m.logger.Printf("lifecycle: on_stopping hook[%d] failed: %v", i, err)
			m.mu.Lock()
			m.shutErr = errors.Join(m.shutErr, fmt.Errorf("lifecycle: on_stopping hook failed: %w", err))
			m.mu.Unlock()
		}
	}

	// Sort by priority descending, matching Start order.
	// The serialExecutor iterates in reverse (LIFO), so the combined
	// effect is: low-priority tasks stop first, high-priority last.
	// This mirrors "defer" semantics — the most critical infrastructure
	// (highest priority) stays alive longest during shutdown.
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})

	// Stop all tasks
	exec := m.buildExecutor()
	if err := exec.Stop(ctx, sorted); err != nil {
		m.logger.Printf("lifecycle: stop errors: %v", err)
		m.mu.Lock()
		m.shutErr = errors.Join(m.shutErr, err)
		m.mu.Unlock()
	}

	// Transition to Stopped
	m.mu.Lock()
	m.state = StateStopped
	m.logger.Printf("lifecycle: manager is now Stopped")
	m.emit(PhaseStopped, "manager", time.Since(stopAt), m.shutErr)

	stoppedHooks := make([]Hook, len(m.onStoppedHooks))
	copy(stoppedHooks, m.onStoppedHooks)
	m.mu.Unlock()

	// Execute OnStopped hooks (outside lock)
	for i, h := range stoppedHooks {
		if err := h(ctx); err != nil {
			m.logger.Printf("lifecycle: on_stopped hook[%d] failed: %v", i, err)
			m.mu.Lock()
			m.shutErr = errors.Join(m.shutErr, fmt.Errorf("lifecycle: on_stopped hook failed: %w", err))
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shutErr
}

// ---------------------------------------------------------------------------
// Blocking helpers
// ---------------------------------------------------------------------------

// Wait blocks until ctx is done, then initiates graceful shutdown with
// the configured timeout. The shutdown reason is ReasonContext.
func (m *Manager) Wait(ctx context.Context) error {
	<-ctx.Done()
	m.logger.Printf("lifecycle: context done (%v), initiating shutdown", ctx.Err())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), m.shutdownTimeout)
	defer cancel()

	return m.ShutdownCtx(shutdownCtx, ReasonContext)
}

// Run is an all-in-one convenience method:
//  1. Starts all tasks (Start)
//  2. Waits for SIGINT or SIGTERM
//  3. Gracefully stops all tasks (ShutdownCtx)
//
// The shutdown timeout is controlled by WithShutdownTimeout.
func (m *Manager) Run() error {
	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("lifecycle: start failed: %w", err)
	}

	if err := m.WaitSignal(m.shutdownTimeout); err != nil {
		return fmt.Errorf("lifecycle: shutdown failed: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Observability
// ---------------------------------------------------------------------------

// Ready returns a channel that is closed when the Manager reaches the
// Running state (i.e., all tasks have started successfully).
func (m *Manager) Ready() <-chan struct{} {
	return m.readyCh
}

// State returns the current lifecycle state.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Reason returns the shutdown reason. Only meaningful after State() == StateStopped.
func (m *Manager) Reason() ShutdownReason {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reason
}

// Err returns the accumulated error from the shutdown process, if any.
func (m *Manager) Err() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.shutErr
}

// ---------------------------------------------------------------------------
// Hook registration
// ---------------------------------------------------------------------------

// OnStart registers a hook executed before tasks are started.
// If any OnStart hook fails, Start returns an error and no tasks are started.
func (m *Manager) OnStart(h Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStartHooks = append(m.onStartHooks, h)
}

// OnReady registers a hook executed after all tasks have started successfully.
// OnReady hook failures are logged but do not prevent the Manager from running.
func (m *Manager) OnReady(h Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onReadyHooks = append(m.onReadyHooks, h)
}

// OnStopping registers a hook executed when shutdown begins, before tasks
// are stopped. Failures are accumulated into the shutdown error.
func (m *Manager) OnStopping(h Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStoppingHooks = append(m.onStoppingHooks, h)
}

// OnStopped registers a hook executed after all tasks have been stopped.
// Failures are accumulated into the shutdown error.
func (m *Manager) OnStopped(h Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStoppedHooks = append(m.onStoppedHooks, h)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (m *Manager) buildExecutor() Executor {
	if m.customExecutor != nil {
		return m.customExecutor
	}
	switch m.executorType {
	case ExecutorParallel:
		return newParallelExecutor(m.logger, m.panicRecovery)
	default:
		return newSerialExecutor(m.logger, m.panicRecovery)
	}
}

// initialized reports whether the Manager was properly created via NewManager.
// A zero-value Manager has nil channels and is not safe to use.
func (m *Manager) initialized() bool {
	return m.readyCh != nil
}

func (m *Manager) emit(phase Phase, name string, d time.Duration, err error) {
	m.metricsHook(MetricEvent{Phase: phase, TaskName: name, Duration: d, Error: err})
}

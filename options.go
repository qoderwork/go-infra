package lifecycle

import "time"

// Option configures a Manager.
type Option func(*Manager)

// WithExecutor sets the task execution strategy.
//
//	mgr := lifecycle.NewManager(lifecycle.WithExecutor(lifecycle.ExecutorParallel))
func WithExecutor(e ExecutorType) Option {
	return func(m *Manager) { m.executorType = e }
}

// WithCustomExecutor injects a user-provided Executor implementation,
// bypassing the built-in serial/parallel selection via WithExecutor.
//
// The Executor interface is exported; this option allows custom scheduling
// strategies (e.g., rate-limited concurrency, dependency graphs).
//
//	mgr := lifecycle.NewManager(lifecycle.WithCustomExecutor(myExecutor))
func WithCustomExecutor(e Executor) Option {
	return func(m *Manager) {
		if e != nil {
			m.customExecutor = e
		}
	}
}

// WithLogger sets a custom logger. The default uses slog.Default().
func WithLogger(l Logger) Option {
	return func(m *Manager) {
		if l != nil {
			m.logger = l
		}
	}
}

// WithMetricsHook registers a callback for lifecycle metric events.
func WithMetricsHook(h MetricsHook) Option {
	return func(m *Manager) {
		if h != nil {
			m.metricsHook = h
		}
	}
}

// WithShutdownTimeout sets the default timeout used by Run() for graceful
// shutdown. Individual ShutdownCtx calls may specify their own timeout.
//
// Default: 30 seconds.
func WithShutdownTimeout(d time.Duration) Option {
	return func(m *Manager) {
		if d > 0 {
			m.shutdownTimeout = d
		}
	}
}

// WithPanicRecovery enables or disables panic recovery during task execution.
// When enabled (the default), a panicking task is recovered and converted
// to an error. When disabled, the panic propagates normally.
//
// Default: true.
func WithPanicRecovery(enabled bool) Option {
	return func(m *Manager) { m.panicRecovery = enabled }
}

// ---------------------------------------------------------------------------
// Executor type enum (convenience for WithExecutor)
// ---------------------------------------------------------------------------

// ExecutorType selects the built-in executor strategy.
type ExecutorType int

const (
	// ExecutorSerial processes tasks one at a time (default).
	ExecutorSerial ExecutorType = iota

	// ExecutorParallel processes all tasks concurrently.
	ExecutorParallel
)

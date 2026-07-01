package lifecycle

import (
	"context"
	"log/slog"
	"time"
)

// Option configures a Manager.
type Option func(*Manager)

// WithTimeout sets the default shutdown timeout.
// Default: 30 seconds.
func WithTimeout(d time.Duration) Option {
	return func(m *Manager) {
		if d > 0 {
			m.timeout = d
		}
	}
}

// WithLogger sets a custom logger.
// Default: slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(m *Manager) {
		if l != nil {
			m.logger = l
		}
	}
}

// ---------------------------------------------------------------------------
// FuncTask — convenience adapter
// ---------------------------------------------------------------------------

// FuncTaskOption configures a FuncTask.
type FuncTaskOption func(*FuncTask)

// FuncTask implements Task using plain functions.
//
//	task := lifecycle.NewFuncTask("redis",
//	    func(ctx context.Context) error { return redis.Connect() },
//	    func(ctx context.Context) error { return redis.Close() },
//	)
type FuncTask struct {
	name    string
	startFn func(context.Context) error
	stopFn  func(context.Context) error
	timeout time.Duration
}

// NewFuncTask creates a Task from start/stop functions.
// Either function may be nil (becomes a no-op).
func NewFuncTask(name string, start, stop func(context.Context) error, opts ...FuncTaskOption) *FuncTask {
	t := &FuncTask{name: name, startFn: start, stopFn: stop}
	for _, o := range opts {
		o(t)
	}
	return t
}

// WithTaskTimeout sets a per-operation timeout.
func WithTaskTimeout(d time.Duration) FuncTaskOption {
	return func(t *FuncTask) { t.timeout = d }
}

func (f *FuncTask) Name() string { return f.name }

func (f *FuncTask) Start(ctx context.Context) error {
	if f.startFn == nil {
		return nil
	}
	if f.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}
	return f.startFn(ctx)
}

func (f *FuncTask) Stop(ctx context.Context) error {
	if f.stopFn == nil {
		return nil
	}
	if f.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}
	return f.stopFn(ctx)
}

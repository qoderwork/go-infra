package lifecycle

import (
	"fmt"
	"log/slog"
)

// Logger is the interface used by the lifecycle Manager for internal logging.
//
// It is intentionally minimal so that *slog.Logger, *log.Logger, and any
// custom implementation all satisfy it without adapters.
type Logger interface {
	Printf(format string, v ...any)
}

// ---------------------------------------------------------------------------
// Built-in logger implementations
// ---------------------------------------------------------------------------

// slogLogger wraps a *slog.Logger to satisfy the Logger interface.
type slogLogger struct{ l *slog.Logger }

func (s *slogLogger) Printf(format string, v ...any) {
	s.l.Info(fmt.Sprintf(format, v...))
}

// defaultLogger returns a Logger backed by slog.Default().
func defaultLogger() Logger {
	return &slogLogger{l: slog.Default()}
}

// NopLogger returns a Logger that discards all output.
// Useful in tests or when metrics/hooks handle all observability.
func NopLogger() Logger { return nopLogger{} }

type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

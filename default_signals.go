//go:build !windows

package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// WaitSignal blocks until one of the specified OS signals is received
// (default: SIGINT, SIGTERM on Unix), then initiates graceful shutdown.
//
// On non-Windows platforms, the default signals are syscall.SIGINT and
// syscall.SIGTERM. On Windows, only os.Interrupt is used as the default.
func (m *Manager) WaitSignal(timeout time.Duration, sigs ...os.Signal) error {
	if len(sigs) == 0 {
		sigs = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	ctx, stop := signal.NotifyContext(context.Background(), sigs...)
	defer stop()

	<-ctx.Done()
	m.logger.Printf("lifecycle: signal received, initiating shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return m.ShutdownCtx(shutdownCtx, ReasonSignal)
}

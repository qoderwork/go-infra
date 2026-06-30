//go:build windows

package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"time"
)

// WaitSignal blocks until one of the specified OS signals is received
// (default: os.Interrupt on Windows), then initiates graceful shutdown.
//
// On non-Windows platforms, the default signals are syscall.SIGINT and
// syscall.SIGTERM. On Windows, only os.Interrupt is used as the default
// because syscall.SIGTERM is not available.
func (m *Manager) WaitSignal(timeout time.Duration, sigs ...os.Signal) error {
	if len(sigs) == 0 {
		sigs = []os.Signal{os.Interrupt}
	}

	ctx, stop := signal.NotifyContext(context.Background(), sigs...)
	defer stop()

	<-ctx.Done()
	m.logger.Printf("lifecycle: signal received, initiating shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return m.ShutdownCtx(shutdownCtx, ReasonSignal)
}

//go:build windows

package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"time"
)

// WaitSignal blocks until os.Interrupt (Ctrl+C),
// then initiates graceful shutdown with the given timeout.
func (m *Manager) WaitSignal(timeout time.Duration, sigs ...os.Signal) error {
	if len(sigs) == 0 {
		sigs = []os.Signal{os.Interrupt}
	}
	ctx, stop := signal.NotifyContext(context.Background(), sigs...)
	defer stop()
	<-ctx.Done()
	m.logger.Info("lifecycle: signal received", "signal", ctx.Err())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return m.Stop(shutdownCtx)
}

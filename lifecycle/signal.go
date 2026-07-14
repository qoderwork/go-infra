package lifecycle

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// WaitSignal blocks until SIGINT or SIGTERM (or the specified signals),
// then initiates graceful shutdown with the given timeout.
func (m *Manager) WaitSignal(timeout time.Duration, sigs ...os.Signal) error {
	if len(sigs) == 0 {
		sigs = []os.Signal{os.Interrupt, syscall.SIGTERM}
	}

	ctx, stop := signal.NotifyContext(context.Background(), sigs...)
	defer stop()

	<-ctx.Done()

	m.logger.Info("lifecycle: shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return m.Stop(shutdownCtx)
}

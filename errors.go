package lifecycle

import "errors"

// ShutdownReason describes what triggered the shutdown.
type ShutdownReason string

const (
	// ReasonManual indicates Shutdown was called directly.
	ReasonManual ShutdownReason = "manual"

	// ReasonSignal indicates an OS signal (SIGINT/SIGTERM) triggered shutdown.
	ReasonSignal ShutdownReason = "signal"

	// ReasonContext indicates a parent context cancellation triggered shutdown.
	ReasonContext ShutdownReason = "context"

	// ReasonError indicates a task failure triggered shutdown.
	ReasonError ShutdownReason = "error"
)

// joinErrors joins a slice of errors using errors.Join, returning nil for
// empty slices.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

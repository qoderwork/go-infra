package lifecycle

import "time"

// Phase identifies a lifecycle phase for metrics reporting.
type Phase string

const (
	PhaseStart    Phase = "start"
	PhaseReady    Phase = "ready"
	PhaseStopping Phase = "stopping"
	PhaseStopped  Phase = "stopped"
)

// MetricEvent describes a lifecycle occurrence suitable for external
// monitoring systems such as Prometheus.
type MetricEvent struct {
	// Phase is the lifecycle phase (start, ready, stopping, stopped).
	Phase Phase

	// TaskName is the name of the task involved, or "manager" for
	// manager-level events.
	TaskName string

	// Duration is the wall-clock time spent in this phase.
	// For PhaseStart: zero (marks the beginning of startup).
	// For PhaseReady: time from Start() call to all tasks running.
	// For PhaseStopping: zero (marks the beginning of shutdown).
	// For PhaseStopped: time from shutdown init to all tasks stopped.
	Duration time.Duration

	// Error is non-nil when the event represents a failure.
	Error error
}

// MetricsHook is a callback invoked for every MetricEvent.
// Implementations must be safe for concurrent use.
type MetricsHook func(event MetricEvent)

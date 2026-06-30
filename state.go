package lifecycle

// State represents the current state of the lifecycle Manager.
//
// The state machine follows a strict linear progression:
//
//	Created → Running → Stopping → Stopped
//
// Any attempt to perform an invalid state transition returns an error.
type State string

const (
	// StateCreated indicates the Manager has been created but not yet started.
	// Tasks can be added in this state.
	StateCreated State = "Created"

	// StateRunning indicates the Manager has started and all tasks are active.
	StateRunning State = "Running"

	// StateStopping indicates the Manager is in the process of shutting down.
	// Tasks are being stopped according to the configured execution strategy.
	StateStopping State = "Stopping"

	// StateStopped indicates the Manager has fully stopped.
	// All tasks have been terminated and cleanup is complete.
	StateStopped State = "Stopped"
)

// String returns the human-readable name of the state.
func (s State) String() string { return string(s) }

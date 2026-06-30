package lifecycle

import "context"

// Hook is a function executed at a lifecycle phase boundary.
//
// Hooks receive the same context passed to Start or Stop, so they can
// participate in deadlines and cancellation.
type Hook func(ctx context.Context) error

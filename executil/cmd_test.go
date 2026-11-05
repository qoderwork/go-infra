package executil

import (
	"context"
	"testing"

	"github.com/qoderwork/go-infra/assert"
)

// TestExecCommandEcho runs a benign echo through the platform shell on both Unix
// ("sh -c") and Windows ("cmd /c"), asserting the combined output is captured.
// The same args work cross-platform because ExecCommand space-joins them on Unix
// and passes them through on Windows.
func TestExecCommandEcho(t *testing.T) {
	out, err := ExecCommand(context.Background(), []string{"echo", "go-infra-ok"})
	assert.NoError(t, err)
	assert.Contains(t, string(out), "go-infra-ok")
}

// TestExecCommandRespectsContextCancel verifies a pre-canceled context stops the
// command from running and surfaces an error instead of hanging. Both platforms
// route through exec.CommandContext and return the context error.
func TestExecCommandRespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ExecCommand(ctx, []string{"echo", "should-not-run"})
	assert.Error(t, err, "canceled context must abort the command")
	assert.ErrorIs(t, err, context.Canceled)
}

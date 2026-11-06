package executil

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
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

// TestExecCommandFailureNotSwallowed verifies a command that writes output AND
// exits non-zero still reports an error on both platforms. The Windows build
// used to clear the error whenever output was present, silently turning real
// failures into successes.
func TestExecCommandFailureNotSwallowed(t *testing.T) {
	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"echo", "boom", "&&", "exit", "/b", "3"}
	} else {
		args = []string{"echo", "boom; exit 3"} // sh -c "echo boom; exit 3"
	}
	out, err := ExecCommand(context.Background(), args)
	assert.Error(t, err, "non-zero exit with output must return an error")
	var ee *exec.ExitError
	assert.True(t, errors.As(err, &ee), "error should be an *exec.ExitError")
	assert.True(t, strings.Contains(string(out), "boom"), "stdout should still be captured, got %q", string(out))
}

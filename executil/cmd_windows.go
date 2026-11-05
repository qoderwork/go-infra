package executil

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// ExecCommand runs args through the Windows shell (%ComSpec% /c ...), returning
// combined output. args is interpreted by cmd.exe without escaping, so callers
// must sanitize any untrusted input (see the package doc). ctx bounds/aborts the
// command via exec.CommandContext; when it is done the context error is returned
// even if the killed process produced output. A non-zero exit with output is
// otherwise reported as a success (err cleared); a failed exit with no output
// keeps its error.
func ExecCommand(ctx context.Context, args []string) ([]byte, error) {
	newArgs := make([]string, 1, len(args)+1)
	newArgs[0] = "/c"
	newArgs = append(newArgs, args...)
	c := os.Getenv("ComSpec")
	cmd := exec.CommandContext(ctx, c, newArgs...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	s, err := cmd.CombinedOutput()
	if ctxErr := ctx.Err(); ctxErr != nil {
		// Cancellation/deadline takes precedence over the killed-process status
		// so callers see context.Canceled / DeadlineExceeded, matching Unix.
		return s, ctxErr
	}
	if err != nil && strings.HasPrefix(err.Error(), "exit status") && len(s) > 0 {
		err = nil
	}
	return s, err
}

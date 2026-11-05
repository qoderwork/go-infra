//go:build !windows

package executil

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// ExecCommand joins args and runs them through "sh -c", returning combined
// output. args is interpreted by the shell without escaping, so callers must
// sanitize any untrusted input (see the package doc). ctx bounds/aborts the
// command; when it is done the context error is returned in preference to the
// killed-process status, matching the Windows implementation.
func ExecCommand(ctx context.Context, args []string) ([]byte, error) {
	cmdStr := strings.Join(args, " ")
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Env = append(cmd.Env, os.Environ()...)
	s, err := cmd.CombinedOutput()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return s, ctxErr
	}
	return s, err
}

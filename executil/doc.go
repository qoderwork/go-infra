// Package executil runs external commands through the platform shell.
//
// ExecCommand interprets its arguments via the host shell (`sh -c` on Unix,
// `cmd.exe /c` on Windows). It performs no escaping or validation of the
// arguments: any untrusted value passed in can alter the executed command.
// Callers that build args from user input are responsible for sanitizing or
// quoting them, or should invoke the binary directly via os/exec instead.
package executil

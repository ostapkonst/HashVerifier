// Package base holds shared CLI helpers: error reporting, flag/config loading, signal-coordinated runtime, and algorithm-resolution utilities reused by every subcommand.
package base

import "fmt"

// ExitError carries a process exit code alongside an optional error. When returned from a command's RunE, main checks for it via errors.As and os.Exits with the given code.
//
// When Silent is true, the global error handler skips the redundant zerolog "Application failed" log line (used by commands that already wrote the error to stderr).
type ExitError struct {
	Code   int
	Err    error
	Silent bool
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}

	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

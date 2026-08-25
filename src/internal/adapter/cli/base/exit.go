// Package base holds shared CLI helpers used by every subcommand: error reporting, flag/config loading, runtime coordination.
package base

import "fmt"

// ExitError pairs a process exit code with an optional error. main recovers it from RunE via errors.As and exits with the code.
// Silent=true skips zerolog's duplicate "Application failed" line when the caller has already written the error to stderr.
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

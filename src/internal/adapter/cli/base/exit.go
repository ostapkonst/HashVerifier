// Package base holds shared CLI helpers used by every subcommand: error reporting,
// flag/config loading, runtime coordination.
package base

import "fmt"

// exitCodesCommon lists the exit codes every command shares; the codes are mapped app-wide in
// exitmap.go and main.go.
const exitCodesCommon = "Exit codes: 0 on success, 1 on any error, 2 on a recovered panic, 130 when canceled"

// ExitCodesHint is the exit-code summary shared by the commands that install a shutdown handler.
const ExitCodesHint = exitCodesCommon + "."

// ConfigExitCodesHint is the exit-code summary for the config subtree, which reports configuration
// problems with 78.
const ConfigExitCodesHint = exitCodesCommon + ", 78 on a configuration error."

// ExitError pairs a process exit code with an optional error. main recovers it from RunE
// via errors.As and exits with the code.
// Silent=true skips zerolog's duplicate "Application failed" line when the caller has
// already written the error to stderr.
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

// Unwrap exposes the wrapped error so errors.Is and errors.As still reach a sentinel
// carried through the ExitError — the shutdown mapper checks for one after the join.
func (e *ExitError) Unwrap() error {
	return e.Err
}

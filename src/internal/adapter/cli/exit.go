package cmd

import "fmt"

// ExitError carries a process exit code alongside an optional error.
// When returned from a command's RunE, main checks for it via errors.As
// and calls os.Exit with the given code instead of the default exit 1.
//
// When Silent is true, the global error handler skips the redundant zerolog
// "Application failed" log line. Used by commands that already format the
// error for the user via fmt.Fprintf(os.Stderr, ...) — re-logging through
// zerolog would just duplicate the message on stdout.
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

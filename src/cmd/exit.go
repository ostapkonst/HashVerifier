package cmd

import "fmt"

// ExitError carries a process exit code alongside an optional error.
// When returned from a command's RunE, main checks for it via errors.As
// and calls os.Exit with the given code instead of the default exit 1.
type ExitError struct {
	Code int
	Err  error
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

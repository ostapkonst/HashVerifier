package base

import (
	"fmt"
	"os"
)

// ReportError formats a user-facing error (title, optional reason, hint) on stderr and returns *ExitError with the given code, Silent=true to suppress zerolog's duplicate "Application failed" line.
func ReportError(title, hint string, exitCode int, err error) *ExitError {
	fmt.Fprintf(os.Stderr, "Error: %s\n", title)

	if err != nil {
		fmt.Fprintf(os.Stderr, "  Reason: %v\n", err)
	}

	fmt.Fprintln(os.Stderr)

	if hint != "" {
		fmt.Fprintf(os.Stderr, "Hint: %s\n", hint)
	}

	return &ExitError{Code: exitCode, Err: err, Silent: true}
}

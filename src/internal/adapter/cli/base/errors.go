package base

import (
	"fmt"
	"os"
)

// ReportError formats a user-facing error on stderr (Error:/Title:/Reason:/Hint:) and returns *ExitError with the given code.
// Silent=true suppresses zerolog's duplicate "Application failed" line; err stays in ExitError.Err for logging but is not displayed.
func ReportError(title, reason, hint string, exitCode int, err error) *ExitError {
	fmt.Fprintln(os.Stderr, "Error:")

	if title != "" {
		fmt.Fprintf(os.Stderr, "  Title:  %s\n", title)
	}

	if reason != "" {
		fmt.Fprintf(os.Stderr, "  Reason: %s\n", reason)
	}

	if hint != "" {
		fmt.Fprintf(os.Stderr, "  Hint:   %s\n", hint)
	}

	return &ExitError{Code: exitCode, Err: err, Silent: true}
}

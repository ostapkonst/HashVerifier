package base

import (
	"fmt"
	"os"
)

// ReportError formats a user-facing error on stderr using a structured
// Error: / Title: / Reason: / Hint: layout, and returns *ExitError with
// the given code. Silent=true suppresses zerolog's duplicate "Application
// failed" line. err is preserved in ExitError.Err for downstream logging
// but is not displayed separately.
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

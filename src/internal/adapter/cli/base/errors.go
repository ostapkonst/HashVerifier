package base

import (
	"fmt"
	"os"
)

// ReportError writes a formatted Error:/Title:/Reason:/Hint:/Path: block to stderr and returns an
// *ExitError marked Silent so zerolog does not double-log it. path="" skips the Path: line;
// err is kept on ExitError.Err for callers that inspect it, but the Silent flag keeps main
// from logging it again.
func ReportError(title, reason, hint string, path string, exitCode int, err error) *ExitError {
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

	if path != "" {
		fmt.Fprintf(os.Stderr, "  Path:   %s\n", path)
	}

	return &ExitError{Code: exitCode, Err: err, Silent: true}
}

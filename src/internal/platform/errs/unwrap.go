// Package errs unwraps nested error chains and produces user-facing one-line summaries.
package errs

import (
	"context"
	"errors"
	"strings"
	"unicode"
)

func unwrapDeep(err error) error {
	for err != nil {
		switch e := err.(type) {
		case interface{ Unwrap() error }:
			if unwrapped := e.Unwrap(); unwrapped != nil {
				err = unwrapped
				continue
			}
		case interface{ Unwrap() []error }:
			// Take the first non-nil element: the first-occurred error is the root cause and the
			// most useful to surface to the user; Join sites must place that cause FIRST
			// (see DEVELOPMENT.md error-combining order).
			var next error

			for _, wrapped := range e.Unwrap() {
				if wrapped != nil {
					next = wrapped
					break
				}
			}

			// No non-nil peer (empty slice or all-nil): this node is the deepest cause, so return
			// it. Reusing err here would keep the same multi-error and loop forever.
			if next == nil {
				return err
			}

			err = next

			continue
		}

		return err
	}

	return nil
}

func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ".")

	if s == "" {
		return s
	}

	runes := []rune(s)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		s = string(runes)
	}

	return s + "."
}

// UnwrapAndNormalize returns the deepest wrapped error's message, normalized
// (trimmed, sentence-cased, period-terminated).
// Empty string means no error.
func UnwrapAndNormalize(err error) string {
	err = unwrapDeep(err)
	if err == nil {
		return ""
	}

	return normalizeText(err.Error())
}

// IsContextDone reports whether err is or wraps context.Canceled or context.DeadlineExceeded
// under the first-occurred unwrap convention. Treats timeout-driven termination as semantically
// equivalent to a manual cancel, since the run is no longer continuing either way.
// A joined "write failure + cancel" chain therefore classifies as a real failure, not a cancel,
// provided Join sites keep placing the earlier cause first (see DEVELOPMENT.md error-combining order).
func IsContextDone(err error) bool {
	deep := unwrapDeep(err)
	return errors.Is(deep, context.Canceled) || errors.Is(deep, context.DeadlineExceeded)
}

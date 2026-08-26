// Package errs unwraps nested error chains and produces user-facing one-line summaries.
package errs

import (
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
			// Take the last non-nil element: hierarchical fmt.Errorf("%w: %w", outer, inner)
			// stores inner at the end of the slice, so the deepest cause is the last peer.
			// errors.Join peers are arbitrary; the last is as valid as any.
			var picked error

			for _, wrapped := range e.Unwrap() {
				if wrapped != nil {
					picked = wrapped
				}
			}

			if picked != nil {
				err = picked
				continue
			}
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

// UnwrapAndNormalize returns the deepest wrapped error's message, normalized (trimmed, sentence-cased, period-terminated).
// Empty string means no error.
func UnwrapAndNormalize(err error) string {
	err = unwrapDeep(err)
	if err == nil {
		return ""
	}

	return normalizeText(err.Error())
}

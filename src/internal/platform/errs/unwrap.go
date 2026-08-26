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
			// Take the first non-nil element and recurse into it: the first-occurred
			// error is the root cause and the most useful to surface to the user.
			// errors.Join and fmt.Errorf("%w: %w") must place the original cause FIRST
			// in the slice for this to work.
			for _, wrapped := range e.Unwrap() {
				if wrapped != nil {
					err = wrapped
					break
				}
			}
			if err != nil {
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

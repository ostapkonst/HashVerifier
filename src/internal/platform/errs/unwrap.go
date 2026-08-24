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
			found := false

			for _, wrapped := range e.Unwrap() {
				if wrapped != nil {
					err = wrapped
					found = true

					break
				}
			}

			if found {
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

// UnwrapAndNormalize returns the deepest error message in the chain, trimmed, capitalised, and terminated with a period. Empty string means no error.
func UnwrapAndNormalize(err error) string {
	err = unwrapDeep(err)
	if err == nil {
		return ""
	}

	return normalizeText(err.Error())
}

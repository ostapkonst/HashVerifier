// Package hidden classifies paths skipped because the host platform marks them hidden.
// The platform convention itself lives in platform/fs (it is a filesystem property); this
// package owns the sentinel so consumers can tell a hidden skip apart from a user exclude
// or an unrepresentable path.
package hidden

import "errors"

// ErrHidden is the sentinel error returned for a file skipped as hidden; classified as GenSkipped.
var ErrHidden = errors.New("hidden entry skipped")

// IsHiddenError lets callers classify hidden-skip failures without type-asserting against ErrHidden.
func IsHiddenError(err error) bool {
	return err != nil && errors.Is(err, ErrHidden)
}

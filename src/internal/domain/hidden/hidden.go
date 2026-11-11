// Package hidden classifies paths skipped because the host platform marks them hidden.
// The platform convention itself lives in platform/fs (it is a filesystem property); this
// package owns the sentinel that supplies the user-facing skip reason.
package hidden

import "errors"

// ErrHidden is the sentinel error returned for a file skipped as hidden; the generate and
// discover flows turn its text into the result's informational Note.
var ErrHidden = errors.New("hidden entry skipped")

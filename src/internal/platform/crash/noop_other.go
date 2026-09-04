//go:build !linux && !windows && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package crash

import "errors"

// newPlatformSink is a build-tag stub for platforms without an OS log sink;
// the Reporter silently falls back to stderr-only on those targets.
func newPlatformSink() (Sink, error) {
	return nil, errors.New("crash: no platform sink on this OS")
}

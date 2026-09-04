package widgets

import (
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// IsAlive reports whether the GTK window is safe to touch from application code.
// Returns false once the window has begun destruction, so any subsequent widget
// access risks use-after-free. Panics on a nil window: every caller in this codebase
// has a non-nil window by construction, and silently treating nil as "alive" would mask
// caller bugs that forgot to wire the window.
func IsAlive(window *gtk.Window) bool {
	if window == nil {
		panic("widgets.IsAlive: window must not be nil")
	}

	return !window.InDestruction()
}

// IdleAdd posts fn to the GTK main loop's idle queue, but drops it when the window is
// already in destruction. Use it for any callback that touches GTK widgets from a
// non-GTK goroutine; without the destruction check a late-firing idle callback can
// use-after-free widgets during shutdown.
//
// Panics inside fn propagate out of the callback: panics represent unrecoverable
// programmer errors (e.g. MustWidget failures from a corrupt .glade) and continuing
// past them would leave the UI in an inconsistent state. The GTK main loop cannot
// host a Go recover above us; the panic will terminate the process via the Go
// runtime. Callers that need recoverable errors should use plain functions and
// return an error rather than panic.
func IdleAdd(window *gtk.Window, fn func()) {
	glib.IdleAdd(func() {
		if !IsAlive(window) {
			return
		}

		fn()
	})
}

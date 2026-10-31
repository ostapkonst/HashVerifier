package widgets

import (
	"context"

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
// Panics inside fn propagate out of the callback and unwind through gtk_main() back to
// the goroutine that called gtk.Main, where the crash reporter's crash.Go wrapper catches
// them (see docs/DEVELOPMENT.md). Callers that need recoverable errors should use plain
// functions and return an error rather than panic.
func IdleAdd(window *gtk.Window, fn func()) {
	glib.IdleAdd(func() {
		if !IsAlive(window) {
			return
		}

		fn()
	})
}

// IdleAddPriority is IdleAdd at an explicit GLib priority: a lower numeric value runs earlier.
// Progress callers pass PRIORITY_HIGH_IDLE (100) so GTK's redraw (120) and GtkTreeView row
// validation (125) cannot starve them, as they would at the default idle priority (200).
func IdleAddPriority(window *gtk.Window, priority glib.Priority, fn func()) {
	glib.IdleAddPriority(priority, func() {
		if !IsAlive(window) {
			return
		}

		fn()
	})
}

// IdleAddSync posts fn to the GTK idle queue and blocks until it has run (or ctx is canceled),
// giving a stream producer backpressure so table updates cannot queue faster than GTK drains
// them. On an already-destroying window fn is skipped but the call still returns. Must not be
// called on the GTK thread: it would deadlock waiting on an idle only that thread can run.
func IdleAddSync(ctx context.Context, window *gtk.Window, fn func()) {
	done := make(chan struct{})

	glib.IdleAdd(func() {
		defer close(done)

		if !IsAlive(window) {
			return
		}

		fn()
	})

	select {
	case <-done:
	case <-ctx.Done():
	}
}

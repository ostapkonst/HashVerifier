package widgets

import (
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// IsAlive reports whether the GTK window is safe to touch from application code.
// Returns true when window is nil (callers may pass nil for dialogs not bound to a window);
// returns false only when the window has begun destruction and any subsequent widget
// access risks use-after-free.
func IsAlive(window *gtk.Window) bool {
	return window == nil || !window.InDestruction()
}

// IdleAdd posts fn to the GTK main loop's idle queue, but drops it when the window is
// already in destruction. Use it for any callback that touches GTK widgets from a
// non-GTK goroutine; without the destruction check a late-firing idle callback can
// use-after-free widgets during shutdown.
func IdleAdd(window *gtk.Window, fn func()) {
	glib.IdleAdd(func() {
		if !IsAlive(window) {
			return
		}

		fn()
	})
}

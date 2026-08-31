package widgets

import (
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// IdleAdd posts fn to the GTK main loop's idle queue, but drops it when window is already
// in destruction. Use it for any callback that touches GTK widgets from a non-GTK goroutine;
// without the destruction check a late-firing idle callback can use-after-free widgets during
// shutdown.
func IdleAdd(window *gtk.Window, fn func()) {
	glib.IdleAdd(func() {
		if window != nil && window.InDestruction() {
			return
		}

		fn()
	})
}

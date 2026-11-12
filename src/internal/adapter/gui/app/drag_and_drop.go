package app

import (
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
)

// DragAndDrop wires GTK drag-and-drop on the main window to a single-path drop callback: the
// main form fills one file or one directory, so only the first dropped path is routed.
type DragAndDrop struct {
	target *widgets.DropTarget
}

// NewDragAndDrop binds a drag-and-drop dispatcher to the window and the single-path drop callback.
func NewDragAndDrop(window *gtk.Window, onPathDrop func(path string)) *DragAndDrop {
	return &DragAndDrop{
		target: widgets.NewDropTarget(window, func(paths []string) {
			if len(paths) == 0 {
				return
			}

			if len(paths) > 1 {
				log.Warn().Int("paths", len(paths)).Msg("Multiple paths dropped; only the first is used")
			}

			onPathDrop(paths[0])
		}),
	}
}

// Setup sets the window as a URI-list drop destination and connects drag-data-received to the dispatcher.
func (d *DragAndDrop) Setup() {
	d.target.Setup()
}

// SetEnabled enables or disables the window drop target; while a run is active the destination
// advertises no action so the cursor shows that a drop will not be handled.
func (d *DragAndDrop) SetEnabled(enabled bool) {
	d.target.SetEnabled(enabled)
}

// DisableDropOnInputWidgets unsets drag destinations on Entry/TextView so their handlers don't swallow drops.
func (d *DragAndDrop) DisableDropOnInputWidgets(root gtk.IWidget) {
	d.target.DisableDropOnInputWidgets(root)
}

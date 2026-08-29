package app

import (
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
)

// DragAndDrop wires GTK drag-and-drop signals on the main window to file-path resolution and tab auto-selection.
type DragAndDrop struct {
	window       *gtk.Window
	pathResolver *PathResolver
	onPathDrop   func(path string)
}

// NewDragAndDrop binds a drag-and-drop dispatcher to the window, a path resolver, and the drop callback.
func NewDragAndDrop(window *gtk.Window, pathResolver *PathResolver, onPathDrop func(path string)) *DragAndDrop {
	return &DragAndDrop{
		window:       window,
		pathResolver: pathResolver,
		onPathDrop:   onPathDrop,
	}
}

// Setup sets the window as a URI-list drop destination and connects drag-data-received to the dispatch handler.
func (d *DragAndDrop) Setup() {
	targetEntry, err := gtk.TargetEntryNew("text/uri-list", gtk.TARGET_OTHER_APP, 0)
	if err != nil {
		widgets.MustWidget("TargetEntry", "DragAndDrop.Setup", err)
	}

	d.window.DragDestSet(gtk.DEST_DEFAULT_ALL, []gtk.TargetEntry{*targetEntry}, gdk.ACTION_COPY)
	d.window.Connect("drag-data-received", func(
		window *gtk.Window,
		ctx *gdk.DragContext,
		x, y int,
		data *gtk.SelectionData,
		info uint,
		time uint,
	) {
		bytes := data.GetData()
		content := string(bytes)

		lines := strings.Split(strings.TrimSpace(content), "\n")
		if len(lines) == 0 || lines[0] == "" {
			log.Warn().Msg("No valid URIs in drag and drop data")
			return
		}

		filePath, err := URIToFilePath(lines[0])
		if err != nil {
			log.Warn().Err(err).Msg("Failed to convert URI to file path")
			return
		}

		if d.onPathDrop != nil {
			d.onPathDrop(filePath)
		}
	})
}

// DisableDropOnInputWidgets unsets drag destinations on Entry/TextView so their handlers don't swallow drops.
func (d *DragAndDrop) DisableDropOnInputWidgets(root gtk.IWidget) {
	for _, w := range collectInputWidgets(root) {
		dragDestUnset(w)
	}
}

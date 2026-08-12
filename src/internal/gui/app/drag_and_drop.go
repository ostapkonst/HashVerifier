package app

import (
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"
)

type DragAndDrop struct {
	window       *gtk.Window
	pathResolver *PathResolver
	onPathDrop   func(path string)
}

func NewDragAndDrop(window *gtk.Window, pathResolver *PathResolver, onPathDrop func(path string)) *DragAndDrop {
	return &DragAndDrop{
		window:       window,
		pathResolver: pathResolver,
		onPathDrop:   onPathDrop,
	}
}

func (d *DragAndDrop) Setup() {
	targetEntry, err := gtk.TargetEntryNew("text/uri-list", gtk.TARGET_OTHER_APP, 0)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create drag and drop target entry")
		return
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
			log.Warn().Msg("Failed to convert URI to file path")
			return
		}

		if d.onPathDrop != nil {
			d.onPathDrop(filePath)
		}
	})
}

func (d *DragAndDrop) DisableDropOnInputWidgets(root gtk.IWidget) {
	for _, w := range collectInputWidgets(root) {
		dragDestUnset(w)
	}
}

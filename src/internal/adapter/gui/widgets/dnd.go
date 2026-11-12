package widgets

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"
)

// dragActionNone is GDK_ACTION_NONE: the destination accepts no action, so GTK shows the
// no-drop cursor and rejects the drop.
const dragActionNone gdk.DragAction = 0

// DropTarget wires a text/uri-list drop destination on a widget and reports every dropped local path.
type DropTarget struct {
	widget      gtk.IWidget
	onPathsDrop func(paths []string)
	targets     []gtk.TargetEntry
}

// NewDropTarget binds a drop dispatcher to widget; onPathsDrop receives every valid local path from a drop.
func NewDropTarget(widget gtk.IWidget, onPathsDrop func(paths []string)) *DropTarget {
	return &DropTarget{
		widget:      widget,
		onPathsDrop: onPathsDrop,
	}
}

// Setup stores the URI-list target, connects drag-data-received once, and enables drops.
func (d *DropTarget) Setup() {
	targetEntry, err := gtk.TargetEntryNew("text/uri-list", gtk.TARGET_OTHER_APP, 0)
	if err != nil {
		MustWidget("TargetEntry", "DropTarget.Setup", err)
	}

	d.targets = []gtk.TargetEntry{*targetEntry}

	widget := d.widget.ToWidget()
	widget.Connect("drag-data-received", func(
		_ interface{},
		_ *gdk.DragContext,
		_, _ int,
		data *gtk.SelectionData,
		_ uint,
		_ uint,
	) {
		paths := parseURIList(string(data.GetData()))
		if len(paths) == 0 {
			log.Warn().Msg("No valid URIs in drag and drop data")

			return
		}

		if d.onPathsDrop != nil {
			d.onPathsDrop(paths)
		}
	})

	d.SetEnabled(true)
}

// SetEnabled controls whether drops are accepted. When disabled, the destination advertises no
// action, so the cursor shows that a drop will not be handled.
func (d *DropTarget) SetEnabled(enabled bool) {
	action := dragActionNone

	if enabled {
		action = gdk.ACTION_COPY
	}

	d.widget.ToWidget().DragDestSet(gtk.DEST_DEFAULT_ALL, d.targets, action)
}

// DisableDropOnInputWidgets unsets drag destinations on Entry/TextView so their handlers don't swallow drops.
func (d *DropTarget) DisableDropOnInputWidgets(root gtk.IWidget) {
	for _, w := range collectInputWidgets(root) {
		dragDestUnset(w)
	}
}

// parseURIList extracts every valid local path from a text/uri-list payload, skipping blank and
// '#' lines. The payload is split on newlines only; a NUL-separated list is not parsed.
func parseURIList(content string) []string {
	var paths []string

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		path, err := uriToFilePath(line)
		if err != nil {
			log.Warn().Err(err).Str("uri", line).Msg("Failed to convert URI to file path")

			continue
		}

		paths = append(paths, path)
	}

	return paths
}

// uriToFilePath decodes a file:// URI into a local path, handling Windows drive-letter and UNC forms.
func uriToFilePath(uri string) (string, error) {
	uri = strings.TrimRight(strings.TrimSpace(uri), "\r\n")
	if uri == "" {
		return "", fmt.Errorf("empty URI")
	}

	parsedURL, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}

	if parsedURL.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme: %s", parsedURL.Scheme)
	}

	path := parsedURL.Path

	if runtime.GOOS == "windows" {
		switch {
		case parsedURL.Host != "":
			path = `\\` + parsedURL.Host + filepath.FromSlash(path)
		case len(path) > 2 && path[0] == '/' && path[2] == ':':
			path = filepath.FromSlash(path[1:])
		default:
			return "", fmt.Errorf("unsupported file URI on Windows: %s", uri)
		}
	}

	// url.Parse already percent-decodes parsedURL.Path, so decoding it again would fail on
	// filenames that legitimately contain a '%' character.
	return path, nil
}

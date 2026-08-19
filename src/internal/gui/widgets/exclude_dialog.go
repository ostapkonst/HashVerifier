// exclude_dialog.go implements a modal dialog for selecting files and
// directories to exclude from checksum generation.
//
// The dialog displays the top-level entries of the input directory as a flat
// list with checkboxes: checked (included), unchecked (excluded). Selecting a
// directory for exclusion excludes every file nested under it — the
// underlying exclude.Matcher handles prefix matching, so a single top-level
// entry is enough. The OK button label reflects the live exclude count.
//
// State persistence between openings (dialog size) is handled by the caller
// via getter methods; only the excluded paths list is returned from Run on OK.
package widgets

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

// ListStore column indices for the exclude dialog's list model.
const (
	excludeColChecked  = 0 // gboolean — checkbox state (true = included)
	excludeColName     = 1 // gchararray — display name (basename)
	excludeColRelPath  = 2 // gchararray — relative path (used for results)
	excludeColIconName = 3 // gchararray — freedesktop icon name (folder/text-x-generic)
)

// ExcludeDialog is a modal dialog for selecting files and directories to
// exclude from checksum generation.
//
// The dialog shows the top-level entries of inputDir as a flat list with
// checkboxes. By default all entries are checked (included). Unchecking an
// entry excludes it; excluding a directory excludes every file nested under
// it (handled prefix-wise by the exclude.Matcher used during generation).
// Existing exclusions are pre-rendered as unchecked on open. Run returns the
// list of excluded rel-paths on OK, or nil on cancel.
type ExcludeDialog struct {
	dialog          *gtk.Dialog
	treeView        *gtk.TreeView
	store           *gtk.ListStore
	okButton        *gtk.Button
	inputDir        string
	outputFile      string
	lastClickedPath *gtk.TreePath
}

// NewExcludeDialog creates an exclude-selection dialog.
//
// Parameters:
//   - existing: already-excluded rel-paths (trailing '/' for directories),
//     rendered as unchecked on open. Only the top-level component of each
//     path is matched against the list (nested paths are rounded up to their
//     top-level directory).
//   - width, height: dialog size from previous open; 0 uses the default.
//
// Returns nil if the dialog could not be created (an error dialog is shown
// to the user in that case).
func NewExcludeDialog(parent *gtk.Window, title, inputDir, outputFile string, existing []string, width, height int) *ExcludeDialog {
	// Create modal dialog with Cancel/OK buttons; restore size if provided.
	dialog, err := gtk.DialogNewWithButtons(
		title,
		parent,
		gtk.DIALOG_MODAL,
		[]interface{}{"_Cancel", gtk.RESPONSE_CANCEL},
		[]interface{}{"Exclude 0 items", gtk.RESPONSE_OK},
	)
	if err != nil {
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	dialog.SetDefaultSize(480, 600)

	if width > 0 && height > 0 {
		dialog.Resize(width, height)
	}

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	// Build ListStore with 4 columns and a TreeView.
	store, err := gtk.ListStoreNew(
		glib.TYPE_BOOLEAN, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING,
	)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	treeView, err := gtk.TreeViewNewWithModel(store)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	treeView.SetHeadersVisible(false)

	selection, err := treeView.GetSelection()
	if err == nil {
		selection.SetMode(gtk.SELECTION_MULTIPLE)
	}

	d := &ExcludeDialog{
		dialog:     dialog,
		treeView:   treeView,
		store:      store,
		inputDir:   inputDir,
		outputFile: outputFile,
	}

	if err := d.setupColumns(treeView); err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	treeView.Connect("button-press-event", d.onButtonPress)

	// Wrap the tree in a scrolled window; it takes most of the dialog space.
	scrolledWin, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	scrolledWin.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWin.Add(treeView)

	contentArea.PackStart(scrolledWin, true, true, 0)

	bottomBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	bottomBox.SetMarginTop(8)
	bottomBox.SetMarginBottom(8)
	bottomBox.SetMarginStart(8)
	bottomBox.SetMarginEnd(8)

	hintLabel, err := gtk.LabelNew("")
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	hintLabel.SetMarkup("<small>Shift+Click — select range  ·  Ctrl+Click — toggle selected</small>")
	hintLabel.SetHAlign(gtk.ALIGN_START)
	hintLabel.SetHExpand(true)
	bottomBox.PackStart(hintLabel, true, true, 0)

	deselectAllImage, err := gtk.ImageNewFromIconName("list-remove", gtk.ICON_SIZE_BUTTON)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	deselectAllBtn, err := gtk.ButtonNew()
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	deselectAllBtn.SetImage(deselectAllImage)
	deselectAllBtn.Connect("clicked", func() { d.setAllChecked(false) })
	bottomBox.PackStart(deselectAllBtn, false, false, 0)

	selectAllImage, err := gtk.ImageNewFromIconName("list-add", gtk.ICON_SIZE_BUTTON)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	selectAllBtn, err := gtk.ButtonNew()
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	selectAllBtn.SetImage(selectAllImage)
	selectAllBtn.Connect("clicked", func() { d.setAllChecked(true) })
	bottomBox.PackStart(selectAllBtn, false, false, 0)

	contentArea.PackStart(bottomBox, false, false, 0)

	// Retrieve the OK button to update its label with the live exclude count.
	okBtn, err := dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	button, ok := okBtn.(*gtk.Button)
	if !ok {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", "Failed to create exclude dialog: OK widget is not a Button")

		return nil
	}

	d.okButton = button

	// Populate list and apply existing exclusions.
	nodeIters, err := d.buildList()
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	d.applyExistingExclusions(existing, nodeIters)
	d.updateExcludedUI()

	return d
}

// setupColumns configures the TreeView columns:
//   - Column 1: checkbox (CellRendererToggle) + icon (CellRendererPixbuf) +
//     name (CellRendererText). The toggle's "active" attribute is bound to the
//     ListStore column, enabling checkbox display.
//
// The relPath column (excludeColRelPath) is kept in the model as hidden data
// for collecting excluded paths on OK, but is not rendered as a TreeView
// column — with a flat top-level list the basename already conveys the same
// information, and the icon distinguishes files from directories.
func (d *ExcludeDialog) setupColumns(treeView *gtk.TreeView) error {
	cellToggle, err := gtk.CellRendererToggleNew()
	if err != nil {
		return fmt.Errorf("failed to create toggle renderer: %w", err)
	}

	cellToggle.SetActivatable(true)

	cellToggle.Connect("toggled", func(_ *gtk.CellRendererToggle, path string) {
		iter, err := d.store.GetIterFromString(path)
		if err != nil {
			return
		}

		d.onToggle(iter)
	})

	cellIcon, err := gtk.CellRendererPixbufNew()
	if err != nil {
		return fmt.Errorf("failed to create pixbuf renderer: %w", err)
	}

	cellName, err := gtk.CellRendererTextNew()
	if err != nil {
		return fmt.Errorf("failed to create text renderer: %w", err)
	}

	colName, err := gtk.TreeViewColumnNew()
	if err != nil {
		return fmt.Errorf("failed to create name column: %w", err)
	}

	colName.PackStart(cellToggle, false)
	colName.PackStart(cellIcon, false)
	colName.PackStart(cellName, true)
	colName.AddAttribute(cellToggle, "active", excludeColChecked)
	colName.AddAttribute(cellIcon, "icon-name", excludeColIconName)
	colName.AddAttribute(cellName, "text", excludeColName)
	colName.SetSizing(gtk.TREE_VIEW_COLUMN_AUTOSIZE)
	treeView.AppendColumn(colName)

	return nil
}

// buildList reads the top-level entries of inputDir and populates the
// ListStore. All entries start as checked (included). Existing exclusions
// are applied separately by applyExistingExclusions. Returns a map of
// normalized top-level rel-path to iter for all entries.
func (d *ExcludeDialog) buildList() (map[string]*gtk.TreeIter, error) {
	entries, err := os.ReadDir(d.inputDir)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}

	sortDirEntriesByDirFirst(entries)

	nodeIters := make(map[string]*gtk.TreeIter, len(entries))

	for _, entry := range entries {
		fullPath := filepath.Join(d.inputDir, entry.Name())

		if isOutputFile(fullPath, d.outputFile) {
			continue
		}

		isDir := entry.IsDir()

		iter := d.store.Append()

		entryRel := entry.Name()
		if isDir {
			entryRel += "/"
		}

		// All entries default to included (checked=true).
		if err := d.store.SetValue(iter, excludeColChecked, true); err != nil {
			return nil, fmt.Errorf("set checked: %w", err)
		}

		if err := d.store.SetValue(iter, excludeColName, entry.Name()); err != nil {
			return nil, fmt.Errorf("set name: %w", err)
		}

		if err := d.store.SetValue(iter, excludeColRelPath, entryRel); err != nil {
			return nil, fmt.Errorf("set relpath: %w", err)
		}

		iconName := "text-x-generic"
		if isDir {
			iconName = "folder"
		}

		if err := d.store.SetValue(iter, excludeColIconName, iconName); err != nil {
			return nil, fmt.Errorf("set icon: %w", err)
		}

		nodeIters[normalizeExcludePath(entryRel)] = iter
	}

	return nodeIters, nil
}

// applyExistingExclusions replays existing exclusions onto the built list,
// simulating a sequence of user unchecks. For each excluded path, its
// top-level component is computed and the matching row is unchecked. Paths
// with no matching top-level entry are silently skipped.
func (d *ExcludeDialog) applyExistingExclusions(existing []string, nodeIters map[string]*gtk.TreeIter) {
	for _, p := range existing {
		topLevel := topLevelComponent(p)
		if topLevel == "" {
			continue
		}

		iter, ok := nodeIters[topLevel]
		if !ok {
			continue
		}

		_ = d.store.SetValue(iter, excludeColChecked, false)
	}
}

// onToggle handles a checkbox click: flips the checked state and refreshes
// the excluded-items UI. No cascade is needed — the list is flat.
func (d *ExcludeDialog) onToggle(iter *gtk.TreeIter) {
	checked, err := d.boolValue(iter, excludeColChecked)
	if err != nil {
		return
	}

	newChecked := !checked

	if err := d.store.SetValue(iter, excludeColChecked, newChecked); err != nil {
		return
	}

	d.updateExcludedUI()
}

// setCheckbox sets a node to a specific checked state (without inversion).
// Used by Shift/Ctrl+click bulk operations.
func (d *ExcludeDialog) setCheckbox(iter *gtk.TreeIter, checked bool) {
	_ = d.store.SetValue(iter, excludeColChecked, checked)
}

// setAllChecked sets every row's checked state to the given value and
// refreshes the OK-button label to reflect the new excluded count.
func (d *ExcludeDialog) setAllChecked(checked bool) {
	iter, ok := d.store.GetIterFirst()
	for ok {
		d.setCheckbox(iter, checked)
		ok = d.store.IterNext(iter)
	}

	d.updateExcludedUI()
}

// onButtonPress handles Shift+click (range) and Ctrl+click (point) bulk
// checkbox toggling. A plain click without modifiers is passed through to
// the CellRendererToggle for normal single-row toggling.
//
// Shift/Ctrl+click on an unselected row extends the selection without
// changing checkboxes. Shift/Ctrl+click on an already-selected row applies
// the inverted state of the clicked row to all selected rows.
func (d *ExcludeDialog) onButtonPress(_ *gtk.TreeView, event *gdk.Event) bool {
	eventButton := gdk.EventButtonNewFromEvent(event)
	if eventButton.Button() != 1 {
		return false
	}

	path, _, _, _, ok := d.treeView.GetPathAtPos(int(eventButton.X()), int(eventButton.Y()))
	if !ok {
		return false
	}

	state := eventButton.State()
	shift := state&uint(gdk.SHIFT_MASK) != 0
	ctrl := state&uint(gdk.CONTROL_MASK) != 0

	if !shift && !ctrl {
		d.lastClickedPath = path

		return false
	}

	selection, err := d.treeView.GetSelection()
	if err != nil {
		return true
	}

	alreadySelected := selection.PathIsSelected(path)

	if alreadySelected {
		d.applySelectionToClickedState(path)
	} else {
		if shift {
			if d.lastClickedPath != nil {
				selection.SelectRange(d.lastClickedPath, path)
			} else {
				selection.SelectPath(path)
			}
		} else {
			selection.SelectPath(path)
		}
	}

	return true
}

// applySelectionToClickedState reads the clicked row's current checked state,
// computes the target (inverted), and applies it to all selected rows.
func (d *ExcludeDialog) applySelectionToClickedState(clickedPath *gtk.TreePath) {
	clickedIter, err := d.store.GetIter(clickedPath)
	if err != nil {
		return
	}

	clickedChecked, err := d.boolValue(clickedIter, excludeColChecked)
	if err != nil {
		return
	}

	targetChecked := !clickedChecked

	selection, err := d.treeView.GetSelection()
	if err != nil {
		return
	}

	selection.SelectedForEach(func(_ *gtk.TreeModel, _ *gtk.TreePath, iter *gtk.TreeIter) {
		d.setCheckbox(iter, targetChecked)
	})

	d.updateExcludedUI()
}

// updateExcludedUI recalculates excluded paths and updates the OK button
// label. Called on every toggle to keep the count in sync.
func (d *ExcludeDialog) updateExcludedUI() {
	count := len(collectExcludedPaths(d.store))
	d.okButton.SetLabel(fmt.Sprintf("Exclude %d items", count))
}

// Run displays the dialog and returns the list of excluded rel-paths.
// The bool is true if the user confirmed with OK, false on cancel or
// window close. On OK the slice may be empty (all entries included) or
// contain rel-paths with trailing '/' for directories.
func (d *ExcludeDialog) Run() ([]string, bool) {
	d.dialog.ShowAll()

	resp := d.dialog.Run()
	if resp != gtk.RESPONSE_OK {
		return nil, false
	}

	return collectExcludedPaths(d.store), true
}

// Destroy releases the dialog's resources.
func (d *ExcludeDialog) Destroy() {
	d.dialog.Destroy()
}

// GetSize returns the current dialog width and height. Must be called
// before Destroy.
func (d *ExcludeDialog) GetSize() (int, int) {
	return d.dialog.GetSize()
}

// boolValue reads a boolean column value for iter.
func (d *ExcludeDialog) boolValue(iter *gtk.TreeIter, col int) (bool, error) {
	val, err := d.store.GetValue(iter, col)
	if err != nil {
		return false, err
	}

	return boolFromValue(val)
}

// collectExcludedPaths traverses the list and collects rel-paths of excluded
// (unchecked) entries. Each unchecked row contributes its rel-path (with a
// trailing '/' for directories, as stored in the relPath column).
func collectExcludedPaths(store *gtk.ListStore) []string {
	var paths []string

	iter, ok := store.GetIterFirst()
	for ok {
		checked, err := boolValueStore(store, iter, excludeColChecked)
		if err == nil && !checked {
			relPath, err := stringValueStore(store, iter, excludeColRelPath)
			if err == nil && relPath != "" {
				paths = append(paths, relPath)
			}
		}

		ok = store.IterNext(iter)
	}

	return paths
}

// boolValueStore reads a boolean column from store for iter. Standalone
// variant of boolValue for use in free functions.
func boolValueStore(store *gtk.ListStore, iter *gtk.TreeIter, col int) (bool, error) {
	val, err := store.GetValue(iter, col)
	if err != nil {
		return false, err
	}

	return boolFromValue(val)
}

// stringValueStore reads a string column from store for iter.
func stringValueStore(store *gtk.ListStore, iter *gtk.TreeIter, col int) (string, error) {
	val, err := store.GetValue(iter, col)
	if err != nil {
		return "", err
	}

	v, err := val.GoValue()
	if err != nil {
		return "", err
	}

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", v)
	}

	return s, nil
}

// boolFromValue extracts a bool from a glib.Value, returning an error if
// the underlying type is not bool.
func boolFromValue(val *glib.Value) (bool, error) {
	v, err := val.GoValue()
	if err != nil {
		return false, err
	}

	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("expected bool, got %T", v)
	}

	return b, nil
}

// normalizeExcludePath converts a rel-path to canonical form: backslashes
// are replaced with forward slashes and the path is cleaned via filepath.Clean.
// Used as a key in nodeIters for cross-platform matching.
func normalizeExcludePath(p string) string {
	if p == "" {
		return ""
	}

	p = strings.ReplaceAll(p, `\`, "/")
	p = filepath.Clean(p)

	return p
}

// topLevelComponent returns the top-level component of a normalized rel-path.
// For "build/" → "build"; for "build/sub/file.log" → "build"; for "secrets.env"
// → "secrets.env". Empty and "." inputs return "".
func topLevelComponent(relPath string) string {
	normalized := normalizeExcludePath(relPath)
	if normalized == "" || normalized == "." {
		return ""
	}

	if idx := strings.Index(normalized, "/"); idx >= 0 {
		return normalized[:idx]
	}

	return normalized
}

// isOutputFile reports whether fullPath is the output checksum file, so it
// can be hidden from the exclude list.
func isOutputFile(fullPath, outputFile string) bool {
	if outputFile == "" {
		return false
	}

	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return false
	}

	absOutput, err := filepath.Abs(outputFile)
	if err != nil {
		return false
	}

	return checksum.PathsEqual(absFull, absOutput)
}

// sortDirEntriesByDirFirst sorts the slice in-place so that directories come
// first, then files, with alphabetical order within each group.
func sortDirEntriesByDirFirst(entries []os.DirEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		iDir := entries[i].IsDir()
		jDir := entries[j].IsDir()

		if iDir != jDir {
			return iDir
		}

		return entries[i].Name() < entries[j].Name()
	})
}

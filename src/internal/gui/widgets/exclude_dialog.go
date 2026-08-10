// exclude_dialog.go implements a modal dialog for selecting files and
// directories to exclude from checksum generation.
//
// The dialog displays the input directory as a collapsible tree with
// tri-state checkboxes: checked (included), unchecked (excluded), and
// inconsistent (partially excluded — shown as a dash). Toggling a directory
// cascades to all descendants. A collapsible panel below the tree lists the
// currently excluded paths in real time.
//
// State persistence between openings (expansion, dialog size, expander state)
// is handled by the caller via getter methods; only the excluded paths list
// is returned from Run on OK.
package widgets

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

// TreeStore column indices for the exclude dialog's tree model.
const (
	excludeColChecked      = 0 // gboolean — checkbox state (true = included)
	excludeColName         = 1 // gchararray — display name (basename)
	excludeColRelPath      = 2 // gchararray — relative path (used for results)
	excludeColIsDir        = 3 // gboolean — whether the node is a directory
	excludeColIconName     = 4 // gchararray — freedesktop icon name (folder/text-x-generic)
	excludeColInconsistent = 5 // gboolean — tri-state indeterminate flag (partial selection)
)

// ExcludeDialog is a modal dialog for selecting files and directories to
// exclude from checksum generation.
//
// The dialog shows inputDir as a tree with tri-state checkboxes. By default
// all nodes are checked (included). Unchecking a node excludes it; toggling
// a directory cascades to all descendants. Existing exclusions are
// pre-rendered as unchecked on open. Run returns the list of excluded
// rel-paths on OK, or nil on cancel.
type ExcludeDialog struct {
	dialog            *gtk.Dialog
	treeView          *gtk.TreeView
	store             *gtk.TreeStore
	okButton          *gtk.Button
	inputDir          string
	outputFile        string
	expanderExcluded  *gtk.Expander
	listStoreExcluded *gtk.ListStore
	lastClickedPath   *gtk.TreePath
}

// NewExcludeDialog creates an exclude-selection dialog.
//
// Parameters:
//   - existing: already-excluded rel-paths (trailing '/' for directories),
//     rendered as unchecked on open.
//   - expandedDirs: rel-paths of directories to expand (from previous open).
//   - expanderExpanded: whether the bottom "Excluded items" panel was expanded.
//   - width, height: dialog size from previous open; 0 uses the default.
//
// Returns nil if the dialog could not be created (an error dialog is shown
// to the user in that case).
func NewExcludeDialog(parent *gtk.Window, title, inputDir string, outputFile string, existing, expandedDirs []string, expanderExpanded bool, width, height int) *ExcludeDialog {
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

	dialog.SetDefaultSize(530, 600)

	if width > 0 && height > 0 {
		dialog.Resize(width, height)
	}

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	// Build TreeStore with 6 columns and a TreeView showing expanders.
	store, err := gtk.TreeStoreNew(
		glib.TYPE_BOOLEAN, glib.TYPE_STRING, glib.TYPE_STRING,
		glib.TYPE_BOOLEAN, glib.TYPE_STRING, glib.TYPE_BOOLEAN,
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
	treeView.SetShowExpanders(true)

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

	// Build the collapsible "Excluded items" panel below the tree.
	expander, err := gtk.ExpanderNew("Excluded items (0)")
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	expander.SetExpanded(false)

	expanderCSS, err := gtk.CssProviderNew()
	if err == nil {
		_ = expanderCSS.LoadFromData(`.exclude-expander arrow { margin-left: 4px; }`)

		screen := dialog.GetScreen()
		gtk.AddProviderForScreen(screen, expanderCSS, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
	}

	styleCtx, err := expander.GetStyleContext()
	if err == nil {
		styleCtx.AddClass("exclude-expander")
	}

	expander.Connect("notify::expanded", func() {
		if expander.GetExpanded() {
			expander.SetMarginTop(0)
		} else {
			expander.SetMarginTop(8)
		}
	})

	if !expander.GetExpanded() {
		expander.SetMarginTop(8)
	}

	listStoreExcluded, err := gtk.ListStoreNew(glib.TYPE_STRING)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	treeExcluded, err := gtk.TreeViewNewWithModel(listStoreExcluded)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	treeExcluded.SetHeadersVisible(false)

	cellExcluded, err := gtk.CellRendererTextNew()
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	colExcluded, err := gtk.TreeViewColumnNewWithAttribute("Path", cellExcluded, "text", 0)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	treeExcluded.AppendColumn(colExcluded)

	scrolledExcluded, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	scrolledExcluded.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledExcluded.SetMinContentHeight(120)
	scrolledExcluded.Add(treeExcluded)

	expander.Add(scrolledExcluded)

	contentArea.PackStart(expander, false, true, 0)

	hintLabel, err := gtk.LabelNew("")
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	hintLabel.SetMarkup("<small>Shift+Click — select range  ·  Ctrl+Click — toggle selected</small>")
	hintLabel.SetHAlign(gtk.ALIGN_START)
	hintLabel.SetMarginTop(8)
	hintLabel.SetMarginStart(8)
	hintLabel.SetMarginEnd(8)
	contentArea.PackStart(hintLabel, false, false, 0)

	d.expanderExcluded = expander
	d.listStoreExcluded = listStoreExcluded

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

	existingSet := make(map[string]struct{}, len(existing))
	for _, p := range existing {
		existingSet[normalizeExcludePath(p)] = struct{}{}
	}

	// Populate tree, apply existing exclusions, restore expansion and expander state.
	nodeIters, err := d.buildTree()
	if err != nil {
		dialog.Destroy()
		ShowError(parent, "Exclude Dialog Error", fmt.Sprintf("Failed to create exclude dialog: %v", err))

		return nil
	}

	d.applyExistingExclusions(existingSet, nodeIters)

	treeView.CollapseAll()
	d.restoreExpansion(expandedDirs, nodeIters)
	d.expanderExcluded.SetExpanded(expanderExpanded)
	d.updateExcludedUI()

	return d
}

// setupColumns configures the TreeView columns:
//   - Column 1: checkbox (CellRendererToggle) + icon (CellRendererPixbuf) +
//     name (CellRendererText). The toggle's "active" and "inconsistent"
//     attributes are bound to the TreeStore columns, enabling tri-state display.
//   - Column 2: relative path (CellRendererText), shown as secondary info.
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
	colName.AddAttribute(cellToggle, "inconsistent", excludeColInconsistent)
	colName.AddAttribute(cellIcon, "icon-name", excludeColIconName)
	colName.AddAttribute(cellName, "text", excludeColName)
	colName.SetSizing(gtk.TREE_VIEW_COLUMN_AUTOSIZE)
	treeView.AppendColumn(colName)

	cellPath, err := gtk.CellRendererTextNew()
	if err != nil {
		return fmt.Errorf("failed to create path renderer: %w", err)
	}

	colPath, err := gtk.TreeViewColumnNew()
	if err != nil {
		return fmt.Errorf("failed to create path column: %w", err)
	}

	colPath.PackStart(cellPath, true)
	colPath.AddAttribute(cellPath, "text", excludeColRelPath)
	colPath.SetSizing(gtk.TREE_VIEW_COLUMN_AUTOSIZE)
	treeView.AppendColumn(colPath)

	return nil
}

// buildTree walks inputDir and populates the TreeStore.
//
// Child nodes are appended to their parent iter; dirIters provides O(1)
// parent lookup by rel-path. All nodes start as checked (included).
// Existing exclusions are applied separately by applyExistingExclusions.
// Returns a map of normalized rel-path to iter for all nodes.
func (d *ExcludeDialog) buildTree() (map[string]*gtk.TreeIter, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	entries, err := collectTreeEntries(ctx, d.inputDir)
	if err != nil {
		return nil, fmt.Errorf("walk dir: %w", err)
	}

	nodeIters := make(map[string]*gtk.TreeIter)
	dirIters := make(map[string]*gtk.TreeIter)

	for _, fullPath := range entries {
		if isOutputFile(fullPath, d.outputFile) {
			continue
		}

		relPath, err := filepath.Rel(d.inputDir, fullPath)
		if err != nil {
			continue
		}

		relPath = filepath.ToSlash(relPath)

		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		isDir := info.IsDir()
		parentRel := filepath.ToSlash(filepath.Dir(relPath))

		if parentRel == "." {
			parentRel = ""
		}

		parent, ok := dirIters[parentRel]
		if !ok {
			parent = nil
		}

		iter := d.store.Append(parent)

		entry := relPath
		if isDir {
			entry = relPath + "/"
		}

		// All nodes default to included (checked=true, inconsistent=false).
		// Exclusions are applied later by applyExistingExclusions.
		if err := d.store.SetValue(iter, excludeColChecked, true); err != nil {
			return nil, fmt.Errorf("set checked: %w", err)
		}

		if err := d.store.SetValue(iter, excludeColName, filepath.Base(fullPath)); err != nil {
			return nil, fmt.Errorf("set name: %w", err)
		}

		if err := d.store.SetValue(iter, excludeColRelPath, entry); err != nil {
			return nil, fmt.Errorf("set relpath: %w", err)
		}

		if err := d.store.SetValue(iter, excludeColIsDir, isDir); err != nil {
			return nil, fmt.Errorf("set isdir: %w", err)
		}

		iconName := "text-x-generic"
		if isDir {
			iconName = "folder"
		}

		if err := d.store.SetValue(iter, excludeColIconName, iconName); err != nil {
			return nil, fmt.Errorf("set icon: %w", err)
		}

		if err := d.store.SetValue(iter, excludeColInconsistent, false); err != nil {
			return nil, fmt.Errorf("set inconsistent: %w", err)
		}

		nodeIters[normalizeExcludePath(entry)] = iter

		if isDir {
			dirIters[relPath] = iter
		}
	}

	return nodeIters, nil
}

// applyExistingExclusions replays existing exclusions onto the built tree,
// simulating a sequence of user unchecks. For each excluded path: sets
// unchecked, clears inconsistent, cascades to children if it's a directory,
// then updates parent state (including inconsistent) up to the root.
func (d *ExcludeDialog) applyExistingExclusions(existing map[string]struct{}, nodeIters map[string]*gtk.TreeIter) {
	for relPath := range existing {
		iter, ok := nodeIters[relPath]
		if !ok {
			continue
		}

		if err := d.store.SetValue(iter, excludeColChecked, false); err != nil {
			continue
		}

		if err := d.store.SetValue(iter, excludeColInconsistent, false); err != nil {
			continue
		}

		isDir, err := d.boolValue(iter, excludeColIsDir)
		if err == nil && isDir {
			d.cascadeToChildren(iter, false)
		}

		d.updateParentState(iter)
	}
}

// restoreExpansion re-expands directories from a saved list of rel-paths.
//
// Paths are sorted by depth (count of '/') so parents expand before
// children — ExpandRow on a child of a collapsed parent is a no-op.
// Directories that no longer exist (deleted/renamed since last open) are
// silently skipped via the nodeIters lookup.
func (d *ExcludeDialog) restoreExpansion(expandedDirs []string, nodeIters map[string]*gtk.TreeIter) {
	if len(expandedDirs) == 0 {
		return
	}

	sorted := make([]string, len(expandedDirs))
	copy(sorted, expandedDirs)

	sort.Slice(sorted, func(i, j int) bool {
		return strings.Count(sorted[i], "/") < strings.Count(sorted[j], "/")
	})

	for _, relPath := range sorted {
		iter, ok := nodeIters[normalizeExcludePath(relPath)]
		if !ok {
			continue
		}

		path, err := d.store.GetPath(iter)
		if err != nil {
			continue
		}

		d.treeView.ExpandRow(path, false)
	}
}

// onToggle handles a checkbox click: flips the checked state, clears
// inconsistent, cascades to children for directories, updates parent
// state, and refreshes the excluded-items UI.
func (d *ExcludeDialog) onToggle(iter *gtk.TreeIter) {
	checked, err := d.boolValue(iter, excludeColChecked)
	if err != nil {
		return
	}

	newChecked := !checked

	if err := d.store.SetValue(iter, excludeColChecked, newChecked); err != nil {
		return
	}

	if err := d.store.SetValue(iter, excludeColInconsistent, false); err != nil {
		return
	}

	isDir, err := d.boolValue(iter, excludeColIsDir)
	if err != nil {
		return
	}

	if isDir {
		d.cascadeToChildren(iter, newChecked)
	}

	d.updateParentState(iter)
	d.updateExcludedUI()
}

// setCheckbox sets a node to a specific checked state (without inversion),
// clears inconsistent, cascades to children for directories, and updates
// parent state. Used by Shift/Ctrl+click bulk operations.
func (d *ExcludeDialog) setCheckbox(iter *gtk.TreeIter, checked bool) {
	if err := d.store.SetValue(iter, excludeColChecked, checked); err != nil {
		return
	}

	if err := d.store.SetValue(iter, excludeColInconsistent, false); err != nil {
		return
	}

	isDir, err := d.boolValue(iter, excludeColIsDir)
	if err != nil {
		return
	}

	if isDir {
		d.cascadeToChildren(iter, checked)
	}

	d.updateParentState(iter)
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

// cascadeToChildren recursively sets checked and clears inconsistent for
// all descendants of parent.
func (d *ExcludeDialog) cascadeToChildren(parent *gtk.TreeIter, checked bool) {
	var child gtk.TreeIter

	ok := d.store.IterChildren(parent, &child)
	for ok {
		if err := d.store.SetValue(&child, excludeColChecked, checked); err != nil {
			return
		}

		if err := d.store.SetValue(&child, excludeColInconsistent, false); err != nil {
			return
		}

		childIsDir, err := d.boolValue(&child, excludeColIsDir)
		if err == nil && childIsDir {
			d.cascadeToChildren(&child, checked)
		}

		ok = d.store.IterNext(&child)
	}
}

// updateParentState recalculates a parent's checked/inconsistent state from
// its direct children and recursively propagates up to the root.
//
// Tri-state logic (checked = included, unchecked = excluded):
//   - all children checked   -> parent checked,   inconsistent=false
//   - no children checked    -> parent unchecked, inconsistent=false
//   - some children checked  -> parent unchecked, inconsistent=true
//
// Clicking an indeterminate parent makes all children checked (via onToggle).
func (d *ExcludeDialog) updateParentState(iter *gtk.TreeIter) {
	parent := new(gtk.TreeIter)
	if !d.store.IterParent(parent, iter) {
		return
	}

	allChecked, anyChecked, hasChildren := d.childrenState(parent)
	if !hasChildren {
		return
	}

	var newChecked, newInconsistent bool

	switch {
	case allChecked:
		newChecked = true
		newInconsistent = false
	case !anyChecked:
		newChecked = false
		newInconsistent = false
	default:
		newChecked = false
		newInconsistent = true
	}

	if err := d.store.SetValue(parent, excludeColChecked, newChecked); err != nil {
		return
	}

	if err := d.store.SetValue(parent, excludeColInconsistent, newInconsistent); err != nil {
		return
	}

	d.updateParentState(parent)
}

// childrenState returns (allChecked, anyChecked, hasChildren) for the direct
// children of iter.
func (d *ExcludeDialog) childrenState(iter *gtk.TreeIter) (bool, bool, bool) {
	var child gtk.TreeIter

	ok := d.store.IterChildren(iter, &child)
	if !ok {
		return false, false, false
	}

	all := true
	any := false

	for ok {
		checked, err := d.boolValue(&child, excludeColChecked)
		inconsistent, _ := d.boolValue(&child, excludeColInconsistent)

		if err == nil && (checked || inconsistent) {
			any = true
		}
		if !checked || inconsistent {
			all = false
		}

		ok = d.store.IterNext(&child)
	}

	return all && any, any, true
}

// updateExcludedUI recalculates excluded paths and updates the OK button
// label, the expander label, and the flat excluded-paths list.
// Called on every toggle to keep all UI in sync.
func (d *ExcludeDialog) updateExcludedUI() {
	paths := collectExcludedPaths(d.store)
	count := len(paths)

	d.okButton.SetLabel(fmt.Sprintf("Exclude %d items", count))
	d.expanderExcluded.SetLabel(fmt.Sprintf("Excluded items (%d)", count))

	d.listStoreExcluded.Clear()

	for _, p := range paths {
		iter := d.listStoreExcluded.Append()
		_ = d.listStoreExcluded.SetValue(iter, 0, p)
	}
}

// Run displays the dialog and returns the list of excluded rel-paths.
// The bool is true if the user confirmed with OK, false on cancel or
// window close. On OK the slice may be empty (all files included) or
// contain rel-paths with trailing '/' for directories.
func (d *ExcludeDialog) Run() ([]string, bool) {
	d.dialog.ShowAll()

	resp := d.dialog.Run()
	if resp != gtk.RESPONSE_OK {
		return nil, false
	}

	return collectExcludedPaths(d.store), true
}

// collectExpandedDirs walks the TreeStore (not the TreeView) and collects
// rel-paths of directories that are visually expanded. Traverses all nodes
// regardless of their own expansion state.
func (d *ExcludeDialog) collectExpandedDirs() []string {
	var dirs []string

	var visit func(iter *gtk.TreeIter)

	visit = func(iter *gtk.TreeIter) {
		isDir, _ := boolValueStore(d.store, iter, excludeColIsDir)
		if !isDir {
			return
		}

		path, err := d.store.GetPath(iter)
		if err == nil && d.treeView.RowExpanded(path) {
			relPath, _ := stringValueStore(d.store, iter, excludeColRelPath)
			if relPath != "" {
				dirs = append(dirs, relPath)
			}
		}

		var child gtk.TreeIter

		ok := d.store.IterChildren(iter, &child)
		for ok {
			visit(&child)
			ok = d.store.IterNext(&child)
		}
	}

	iter, ok := d.store.GetIterFirst()
	for ok {
		visit(iter)
		ok = d.store.IterNext(iter)
	}

	return dirs
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

// ExpandedDirs returns rel-paths of expanded directories. Must be called
// before Destroy.
func (d *ExcludeDialog) ExpandedDirs() []string {
	return d.collectExpandedDirs()
}

// ExpanderExpanded returns the state of the bottom "Excluded items" panel.
// Must be called before Destroy.
func (d *ExcludeDialog) ExpanderExpanded() bool {
	return d.expanderExcluded.GetExpanded()
}

// boolValue reads a boolean column value for iter.
func (d *ExcludeDialog) boolValue(iter *gtk.TreeIter, col int) (bool, error) {
	val, err := d.store.GetValue(iter, col)
	if err != nil {
		return false, err
	}

	return boolFromValue(val)
}

// collectExcludedPaths traverses the tree and collects rel-paths of excluded
// (unchecked) nodes. If a directory is fully excluded (unchecked and not
// inconsistent), its rel-path is added as a single entry. If a directory is
// partially excluded (inconsistent), traversal descends into children and
// collects individual file paths instead.
func collectExcludedPaths(store *gtk.TreeStore) []string {
	var paths []string

	var visit func(iter *gtk.TreeIter)

	visit = func(iter *gtk.TreeIter) {
		checked, err := boolValueStore(store, iter, excludeColChecked)
		if err != nil {
			return
		}

		if !checked {
			inconsistent, _ := boolValueStore(store, iter, excludeColInconsistent)
			isDir, _ := boolValueStore(store, iter, excludeColIsDir)

			if isDir && inconsistent {
				var child gtk.TreeIter

				ok := store.IterChildren(iter, &child)
				for ok {
					visit(&child)
					ok = store.IterNext(&child)
				}

				return
			}

			relPath, err := stringValueStore(store, iter, excludeColRelPath)
			if err == nil && relPath != "" {
				paths = append(paths, relPath)
			}

			return
		}

		isDir, _ := boolValueStore(store, iter, excludeColIsDir)
		if isDir {
			var child gtk.TreeIter

			ok := store.IterChildren(iter, &child)
			for ok {
				visit(&child)
				ok = store.IterNext(&child)
			}
		}
	}

	iter, ok := store.GetIterFirst()
	for ok {
		visit(iter)
		ok = store.IterNext(iter)
	}

	return paths
}

// boolValueStore reads a boolean column from store for iter. Standalone
// variant of boolValue for use in free functions.
func boolValueStore(store *gtk.TreeStore, iter *gtk.TreeIter, col int) (bool, error) {
	val, err := store.GetValue(iter, col)
	if err != nil {
		return false, err
	}

	return boolFromValue(val)
}

// stringValueStore reads a string column from store for iter.
func stringValueStore(store *gtk.TreeStore, iter *gtk.TreeIter, col int) (string, error) {
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
// Used as a key in nodeIters and existingSet for cross-platform matching.
func normalizeExcludePath(p string) string {
	if p == "" {
		return ""
	}

	p = strings.ReplaceAll(p, `\`, "/")
	p = filepath.Clean(p)

	return p
}

// collectTreeEntries walks root and returns absolute paths of all files and
// directories (excluding root itself). Uses filepath.WalkDir which does not
// follow directory symlinks by default — safe from symlink loops.
func collectTreeEntries(ctx context.Context, root string) ([]string, error) {
	var entries []string

	err := filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip inaccessible entries
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if path == root {
			return nil
		}

		entries = append(entries, path)

		return nil
	})

	return entries, err
}

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

	return absFull == absOutput
}

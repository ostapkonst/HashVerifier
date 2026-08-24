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
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
)

const (
	excludeColChecked  = 0 // gboolean — checkbox state (true = included)
	excludeColName     = 1 // gchararray — display name (basename)
	excludeColRelPath  = 2 // gchararray — relative path (used for results)
	excludeColIconName = 3 // gchararray — freedesktop icon name (folder/text-x-generic)
)

// ExcludeDialog is a modal dialog listing top-level entries of inputDir with checkboxes; the OK label reflects the live excluded count.
type ExcludeDialog struct {
	dialog          *gtk.Dialog
	treeView        *gtk.TreeView
	store           *gtk.ListStore
	okButton        *gtk.Button
	inputDir        string
	outputFile      string
	lastClickedPath *gtk.TreePath
}

// NewExcludeDialog builds and shows the modal dialog. Returns nil on failure (an error dialog is shown to the user). existing entries are pre-rendered as unchecked; nested paths are rounded up to their top-level directory.
func NewExcludeDialog(parent *gtk.Window, title, inputDir, outputFile string, existing []string, width, height int) *ExcludeDialog {
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

func (d *ExcludeDialog) setCheckbox(iter *gtk.TreeIter, checked bool) {
	_ = d.store.SetValue(iter, excludeColChecked, checked)
}

func (d *ExcludeDialog) setAllChecked(checked bool) {
	iter, ok := d.store.GetIterFirst()
	for ok {
		d.setCheckbox(iter, checked)
		ok = d.store.IterNext(iter)
	}

	d.updateExcludedUI()
}

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

func (d *ExcludeDialog) updateExcludedUI() {
	count := len(collectExcludedPaths(d.store))
	d.okButton.SetLabel(fmt.Sprintf("Exclude %d items", count))
}

// Run displays the dialog modally and returns the excluded rel-paths (with trailing '/' for directories). The bool is true on OK.
func (d *ExcludeDialog) Run() ([]string, bool) {
	d.dialog.ShowAll()

	resp := d.dialog.Run()
	if resp != gtk.RESPONSE_OK {
		return nil, false
	}

	return collectExcludedPaths(d.store), true
}

// Destroy releases the dialog's resources; must be called before GetSize to read valid dimensions.
func (d *ExcludeDialog) Destroy() {
	d.dialog.Destroy()
}

// GetSize returns the current dialog width and height; must be called before Destroy.
func (d *ExcludeDialog) GetSize() (int, int) {
	return d.dialog.GetSize()
}

func (d *ExcludeDialog) boolValue(iter *gtk.TreeIter, col int) (bool, error) {
	val, err := d.store.GetValue(iter, col)
	if err != nil {
		return false, err
	}

	return boolFromValue(val)
}

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

func boolValueStore(store *gtk.ListStore, iter *gtk.TreeIter, col int) (bool, error) {
	val, err := store.GetValue(iter, col)
	if err != nil {
		return false, err
	}

	return boolFromValue(val)
}

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

func normalizeExcludePath(p string) string {
	if p == "" {
		return ""
	}

	p = strings.ReplaceAll(p, `\`, "/")
	p = filepath.Clean(p)

	return p
}

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

	return hashfn.PathsEqual(absFull, absOutput)
}

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

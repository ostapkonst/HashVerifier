package widgets

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

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

// NewExcludeDialog builds the modal exclude-list dialog: existing entries start unchecked.
// Panics on GTK widget construction failures (fail-fast pattern, consistent with gtk_getters).
func NewExcludeDialog(parent *gtk.Window, title, inputDir, outputFile string, existing []string, width, height int) *ExcludeDialog {
	dialog, err := gtk.DialogNewWithButtons(
		title,
		parent,
		gtk.DIALOG_MODAL,
		[]interface{}{"_Cancel", gtk.RESPONSE_CANCEL},
		[]interface{}{"Exclude 0 items", gtk.RESPONSE_OK},
	)
	if err != nil {
		MustWidget("Dialog", "NewExcludeDialog", err)
	}

	success := false
	defer func() {
		if !success {
			dialog.Destroy()
		}
	}()

	dialog.SetDefaultSize(480, 600)

	if width > 0 && height > 0 {
		dialog.Resize(width, height)
	}

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		MustWidget("ContentArea", "NewExcludeDialog", err)
	}

	store, err := gtk.ListStoreNew(
		glib.TYPE_BOOLEAN, glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING,
	)
	if err != nil {
		MustWidget("ListStore", "NewExcludeDialog", err)
	}

	treeView, err := gtk.TreeViewNewWithModel(store)
	if err != nil {
		MustWidget("TreeView", "NewExcludeDialog", err)
	}

	treeView.SetHeadersVisible(false)

	selection, err := treeView.GetSelection()
	if err != nil {
		MustWidget("TreeView", "NewExcludeDialog:GetSelection", err)
	}

	selection.SetMode(gtk.SELECTION_MULTIPLE)

	d := &ExcludeDialog{
		dialog:     dialog,
		treeView:   treeView,
		store:      store,
		inputDir:   inputDir,
		outputFile: outputFile,
	}

	if err := d.setupColumns(treeView); err != nil {
		MustWidget("ExcludeDialog.Columns", "NewExcludeDialog", err)
	}

	treeView.Connect("button-press-event", d.onButtonPress)

	scrolledWin, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		MustWidget("ScrolledWindow", "NewExcludeDialog", err)
	}

	scrolledWin.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWin.Add(treeView)

	contentArea.PackStart(scrolledWin, true, true, 0)

	bottomBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		MustWidget("Box", "NewExcludeDialog", err)
	}

	bottomBox.SetMarginTop(8)
	bottomBox.SetMarginBottom(8)
	bottomBox.SetMarginStart(8)
	bottomBox.SetMarginEnd(8)

	hintLabel, err := gtk.LabelNew("")
	if err != nil {
		MustWidget("Label", "NewExcludeDialog", err)
	}

	hintLabel.SetMarkup("<small>Shift+Click — select range  ·  Ctrl+Click — toggle selected</small>")
	hintLabel.SetHAlign(gtk.ALIGN_START)
	hintLabel.SetHExpand(true)
	bottomBox.PackStart(hintLabel, true, true, 0)

	deselectAllImage, err := gtk.ImageNewFromIconName("list-remove", gtk.ICON_SIZE_BUTTON)
	if err != nil {
		MustWidget("Image", "NewExcludeDialog:deselectAll", err)
	}

	deselectAllBtn, err := gtk.ButtonNew()
	if err != nil {
		MustWidget("Button", "NewExcludeDialog:deselectAll", err)
	}

	deselectAllBtn.SetImage(deselectAllImage)
	deselectAllBtn.Connect("clicked", func() { d.setAllChecked(false) })
	bottomBox.PackStart(deselectAllBtn, false, false, 0)

	selectAllImage, err := gtk.ImageNewFromIconName("list-add", gtk.ICON_SIZE_BUTTON)
	if err != nil {
		MustWidget("Image", "NewExcludeDialog:selectAll", err)
	}

	selectAllBtn, err := gtk.ButtonNew()
	if err != nil {
		MustWidget("Button", "NewExcludeDialog:selectAll", err)
	}

	selectAllBtn.SetImage(selectAllImage)
	selectAllBtn.Connect("clicked", func() { d.setAllChecked(true) })
	bottomBox.PackStart(selectAllBtn, false, false, 0)

	contentArea.PackStart(bottomBox, false, false, 0)

	okBtn, err := dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
	if err != nil {
		MustWidget("DialogResponse", "NewExcludeDialog", err)
	}

	button, ok := okBtn.(*gtk.Button)
	if !ok {
		MustWidget("Button", "NewExcludeDialog:typeAssert", fmt.Errorf("OK widget is not a Button"))
	}

	d.okButton = button

	nodeIters := d.buildList()

	d.applyExistingExclusions(existing, nodeIters)
	d.updateExcludedUI()

	success = true

	return d
}

func (d *ExcludeDialog) setupColumns(treeView *gtk.TreeView) error {
	cellToggle, err := gtk.CellRendererToggleNew()
	if err != nil {
		MustWidget("CellRendererToggle", "ExcludeDialog.setupColumns", err)
	}

	cellToggle.SetActivatable(true)

	cellToggle.Connect("toggled", func(_ *gtk.CellRendererToggle, path string) {
		iter, err := d.store.GetIterFromString(path)
		if err != nil {
			MustWidget("ListStore", "ExcludeDialog.CellToggle:GetIterFromString", err)
		}

		d.onToggle(iter)
	})

	cellIcon, err := gtk.CellRendererPixbufNew()
	if err != nil {
		MustWidget("CellRendererPixbuf", "ExcludeDialog.setupColumns", err)
	}

	cellName, err := gtk.CellRendererTextNew()
	if err != nil {
		MustWidget("CellRendererText", "ExcludeDialog.setupColumns", err)
	}

	colName, err := gtk.TreeViewColumnNew()
	if err != nil {
		MustWidget("TreeViewColumn", "ExcludeDialog.setupColumns", err)
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

func (d *ExcludeDialog) buildList() map[string]*gtk.TreeIter {
	entries, err := os.ReadDir(d.inputDir)
	if err != nil {
		MustWidget("os.ReadDir", "ExcludeDialog.buildList", err)
	}

	sortDirEntriesByDirFirst(entries)

	nodeIters := make(map[string]*gtk.TreeIter, len(entries))

	for _, entry := range entries {
		fullPath := filepath.Join(d.inputDir, entry.Name())

		isOutput, err := isOutputFile(fullPath, d.outputFile)
		if err != nil {
			log.Warn().Err(err).Str("path", fullPath).Msg("isOutputFile failed; entry shown to user")
		} else if isOutput {
			continue
		}

		isDir := entry.IsDir()

		iter := d.store.Append()

		entryRel := entry.Name()
		if isDir {
			entryRel += "/"
		}

		if err := d.store.SetValue(iter, excludeColChecked, true); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:checked", err)
		}

		if err := d.store.SetValue(iter, excludeColName, entry.Name()); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:name", err)
		}

		if err := d.store.SetValue(iter, excludeColRelPath, entryRel); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:relpath", err)
		}

		iconName := "text-x-generic"
		if isDir {
			iconName = "folder"
		}

		if err := d.store.SetValue(iter, excludeColIconName, iconName); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:icon", err)
		}

		nodeIters[normalizeExcludePath(entryRel)] = iter
	}

	return nodeIters
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

		if err := d.store.SetValue(iter, excludeColChecked, false); err != nil {
			MustWidget("ListStore", "ExcludeDialog.applyExistingExclusions", err)
		}
	}
}

func (d *ExcludeDialog) onToggle(iter *gtk.TreeIter) {
	checked, err := d.boolValue(iter, excludeColChecked)
	if err != nil {
		MustWidget("ListStore", "ExcludeDialog.onToggle:GetValue", err)
	}

	newChecked := !checked

	if err := d.store.SetValue(iter, excludeColChecked, newChecked); err != nil {
		MustWidget("ListStore", "ExcludeDialog.onToggle", err)
	}

	d.updateExcludedUI()
}

func (d *ExcludeDialog) setCheckbox(iter *gtk.TreeIter, checked bool) {
	if err := d.store.SetValue(iter, excludeColChecked, checked); err != nil {
		MustWidget("ListStore", "ExcludeDialog.setCheckbox", err)
	}
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
		MustWidget("TreeView", "ExcludeDialog.onButtonPress:GetSelection", err)
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
		MustWidget("ListStore", "ExcludeDialog.applySelectionToClickedState:GetIter", err)
	}

	clickedChecked, err := d.boolValue(clickedIter, excludeColChecked)
	if err != nil {
		MustWidget("ListStore", "ExcludeDialog.applySelectionToClickedState:GetValue", err)
	}

	targetChecked := !clickedChecked

	selection, err := d.treeView.GetSelection()
	if err != nil {
		MustWidget("TreeView", "ExcludeDialog.applySelectionToClickedState:GetSelection", err)
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

// Destroy releases the dialog's resources; must be called after GetSize so callers can persist the final dimensions.
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
		return false, fmt.Errorf("read bool column %d: %w", col, err)
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
		return false, fmt.Errorf("read bool column %d: %w", col, err)
	}

	return boolFromValue(val)
}

func stringValueStore(store *gtk.ListStore, iter *gtk.TreeIter, col int) (string, error) {
	val, err := store.GetValue(iter, col)
	if err != nil {
		return "", fmt.Errorf("read string column %d: %w", col, err)
	}

	v, err := val.GoValue()
	if err != nil {
		return "", fmt.Errorf("extract Go value from string column %d: %w", col, err)
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
		return false, fmt.Errorf("extract Go value: %w", err)
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

	return caseNormalizeForFS(p)
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

// caseNormalizeForFS returns lowercased s on filesystems that are typically case-insensitive
// (Windows and macOS) so exclude dialog state matches the on-disk casing; other platforms keep s unchanged.
func caseNormalizeForFS(s string) string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(s)
	}

	return s
}

func isOutputFile(fullPath, outputFile string) (bool, error) {
	if outputFile == "" {
		return false, nil
	}

	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return false, fmt.Errorf("resolve full path: %w", err)
	}

	absOutput, err := filepath.Abs(outputFile)
	if err != nil {
		return false, fmt.Errorf("resolve output file path: %w", err)
	}

	return hashfn.PathsEqual(absFull, absOutput), nil
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

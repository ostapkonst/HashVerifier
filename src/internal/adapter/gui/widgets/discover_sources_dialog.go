package widgets

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
)

const (
	discoverColIcon = 0 // gchararray — freedesktop icon name (folder/text-x-generic)
	discoverColPath = 1 // gchararray — absolute path
)

// DiscoverSource is a file or directory selected for discovery scanning.
type DiscoverSource struct {
	Path  string
	IsDir bool
}

// DiscoverOptions is the sources dialog's result: which sources to scan and how.
type DiscoverOptions struct {
	Sources             []DiscoverSource
	Filter              embedded.FilterMode
	FollowSymbolicLinks bool
	SkipHidden          bool
}

// DiscoverSourcesDialog is a modal dialog listing the sources to scan; rows are added with
// the file/folder pickers and removed explicitly with the Remove button.
type DiscoverSourcesDialog struct {
	dialog            *gtk.Dialog
	parent            *gtk.Window
	treeView          *gtk.TreeView
	store             *gtk.ListStore
	selection         *gtk.TreeSelection
	removeButton      *gtk.Button
	okButton          *gtk.Button
	radioAll          *gtk.RadioButton
	radioWith         *gtk.RadioButton
	radioWithout      *gtk.RadioButton
	chkFollowSymlinks *gtk.CheckButton
	chkSkipHidden     *gtk.CheckButton
	dropTarget        *DropTarget
}

// NewDiscoverSourcesDialog builds the modal sources dialog seeded with the previously chosen options.
// Panics on GTK widget construction failures (fail-fast, consistent with gtk_getters).
func NewDiscoverSourcesDialog(parent *gtk.Window, title string, current DiscoverOptions, width, height int) *DiscoverSourcesDialog {
	dialog, err := gtk.DialogNewWithButtons(
		title,
		parent,
		gtk.DIALOG_MODAL,
		[]interface{}{"_Cancel", gtk.RESPONSE_CANCEL},
		[]interface{}{"Discover", gtk.RESPONSE_OK},
	)
	if err != nil {
		MustWidget("Dialog", "NewDiscoverSourcesDialog", err)
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
		MustWidget("ContentArea", "NewDiscoverSourcesDialog", err)
	}

	store, err := gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		MustWidget("ListStore", "NewDiscoverSourcesDialog", err)
	}

	treeView, err := gtk.TreeViewNewWithModel(store)
	if err != nil {
		MustWidget("TreeView", "NewDiscoverSourcesDialog", err)
	}

	treeView.SetEnableSearch(false)
	treeView.SetSearchColumn(-1)
	treeView.SetHeadersVisible(false)

	selection, err := treeView.GetSelection()
	if err != nil {
		MustWidget("TreeView", "NewDiscoverSourcesDialog:GetSelection", err)
	}

	selection.SetMode(gtk.SELECTION_MULTIPLE)

	d := &DiscoverSourcesDialog{
		dialog:    dialog,
		parent:    parent,
		treeView:  treeView,
		store:     store,
		selection: selection,
	}

	if err := d.setupColumns(treeView); err != nil {
		MustWidget("DiscoverSourcesDialog.Columns", "NewDiscoverSourcesDialog", err)
	}

	d.radioAll, err = gtk.RadioButtonNewWithLabel(nil, "All files")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:all", err)
	}

	d.radioWith, err = gtk.RadioButtonNewWithLabelFromWidget(d.radioAll, "With CRC-32 in the name")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:with", err)
	}

	d.radioWithout, err = gtk.RadioButtonNewWithLabelFromWidget(d.radioAll, "Without CRC-32 in the name")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:without", err)
	}

	switch current.Filter {
	case embedded.FilterWithChecksum:
		d.radioWith.SetActive(true)
	case embedded.FilterWithoutChecksum:
		d.radioWithout.SetActive(true)
	default:
		d.radioAll.SetActive(true)
	}

	scrolledWin, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		MustWidget("ScrolledWindow", "NewDiscoverSourcesDialog", err)
	}

	scrolledWin.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWin.Add(treeView)

	contentArea.PackStart(scrolledWin, true, true, 0)

	hintLabel, err := gtk.LabelNew("")
	if err != nil {
		MustWidget("Label", "NewDiscoverSourcesDialog:hint", err)
	}

	hintLabel.SetMarkup("<small>Drag and drop files or folders here to add them.</small>")
	hintLabel.SetMarginTop(8)
	hintLabel.SetMarginBottom(8)
	hintLabel.SetMarginStart(8)
	hintLabel.SetMarginEnd(8)
	hintLabel.SetHAlign(gtk.ALIGN_START)
	contentArea.PackStart(hintLabel, false, false, 0)

	bottomBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		MustWidget("Box", "NewDiscoverSourcesDialog", err)
	}

	bottomBox.SetMarginTop(8)
	bottomBox.SetMarginBottom(8)
	bottomBox.SetMarginStart(8)
	bottomBox.SetMarginEnd(8)

	addFolderBtn, err := gtk.ButtonNewWithLabel("Add Folder…")
	if err != nil {
		MustWidget("Button", "NewDiscoverSourcesDialog:addFolder", err)
	}

	addFolderBtn.Connect("clicked", func() { d.onAddFolder() })
	bottomBox.PackStart(addFolderBtn, false, false, 0)

	addFilesBtn, err := gtk.ButtonNewWithLabel("Add Files…")
	if err != nil {
		MustWidget("Button", "NewDiscoverSourcesDialog:addFiles", err)
	}

	addFilesBtn.Connect("clicked", func() { d.onAddFiles() })
	bottomBox.PackStart(addFilesBtn, false, false, 0)

	d.removeButton, err = gtk.ButtonNewWithLabel("Remove")
	if err != nil {
		MustWidget("Button", "NewDiscoverSourcesDialog:remove", err)
	}

	d.removeButton.SetSensitive(false)
	d.removeButton.Connect("clicked", d.onRemove)
	bottomBox.PackEnd(d.removeButton, false, false, 0)

	contentArea.PackStart(bottomBox, false, false, 0)

	d.chkFollowSymlinks, err = gtk.CheckButtonNewWithLabel("Follow symlinks")
	if err != nil {
		MustWidget("CheckButton", "NewDiscoverSourcesDialog:followSymlinks", err)
	}

	d.chkFollowSymlinks.SetActive(current.FollowSymbolicLinks)

	d.chkSkipHidden, err = gtk.CheckButtonNewWithLabel("Skip hidden")
	if err != nil {
		MustWidget("CheckButton", "NewDiscoverSourcesDialog:skipHidden", err)
	}

	d.chkSkipHidden.SetActive(current.SkipHidden)

	followBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	if err != nil {
		MustWidget("Box", "NewDiscoverSourcesDialog:options", err)
	}

	followBox.SetMarginTop(8)
	followBox.SetMarginBottom(8)
	followBox.SetMarginStart(8)
	followBox.SetMarginEnd(8)
	followBox.PackStart(d.chkFollowSymlinks, false, false, 0)
	followBox.PackStart(d.chkSkipHidden, false, false, 0)

	contentArea.PackStart(followBox, false, false, 0)

	modeBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	if err != nil {
		MustWidget("Box", "NewDiscoverSourcesDialog:mode", err)
	}

	modeBox.SetMarginTop(8)
	modeBox.SetMarginBottom(8)
	modeBox.SetMarginStart(8)
	modeBox.SetMarginEnd(8)
	modeBox.PackStart(d.radioAll, false, false, 0)
	modeBox.PackStart(d.radioWith, false, false, 0)
	modeBox.PackStart(d.radioWithout, false, false, 0)

	contentArea.PackStart(modeBox, false, false, 0)

	selection.Connect("changed", d.updateUI)

	okBtn, err := dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
	if err != nil {
		MustWidget("DialogResponse", "NewDiscoverSourcesDialog", err)
	}

	button, ok := okBtn.(*gtk.Button)
	if !ok {
		MustWidget("Button", "NewDiscoverSourcesDialog:typeAssert", fmt.Errorf("OK widget is not a Button"))
	}

	d.okButton = button

	for _, src := range current.Sources {
		d.appendRow(src.Path, src.IsDir)
	}

	d.dropTarget = NewDropTarget(dialog, d.onPathsDropped)
	d.dropTarget.Setup()
	d.dropTarget.DisableDropOnInputWidgets(dialog)

	d.updateUI()

	success = true

	return d
}

func (d *DiscoverSourcesDialog) setupColumns(treeView *gtk.TreeView) error {
	cellIcon, err := gtk.CellRendererPixbufNew()
	if err != nil {
		MustWidget("CellRendererPixbuf", "DiscoverSourcesDialog.setupColumns", err)
	}

	cellName, err := gtk.CellRendererTextNew()
	if err != nil {
		MustWidget("CellRendererText", "DiscoverSourcesDialog.setupColumns", err)
	}

	colName, err := gtk.TreeViewColumnNew()
	if err != nil {
		MustWidget("TreeViewColumn", "DiscoverSourcesDialog.setupColumns", err)
	}

	colName.PackStart(cellIcon, false)
	colName.PackStart(cellName, true)
	colName.AddAttribute(cellIcon, "icon-name", discoverColIcon)
	colName.AddAttribute(cellName, "text", discoverColPath)
	colName.SetSizing(gtk.TREE_VIEW_COLUMN_AUTOSIZE)
	treeView.AppendColumn(colName)

	return nil
}

func (d *DiscoverSourcesDialog) appendRow(path string, isDir bool) {
	if path == "" || d.hasPath(path) {
		return
	}

	iconName := "text-x-generic"
	if isDir {
		iconName = "folder"
	}

	iter := d.store.Append()

	if err := d.store.SetValue(iter, discoverColIcon, iconName); err != nil {
		MustWidget("ListStore", "DiscoverSourcesDialog.appendRow:icon", err)
	}

	if err := d.store.SetValue(iter, discoverColPath, path); err != nil {
		MustWidget("ListStore", "DiscoverSourcesDialog.appendRow:path", err)
	}

	d.updateUI()
}

func (d *DiscoverSourcesDialog) hasPath(path string) bool {
	iter, ok := d.store.GetIterFirst()
	for ok {
		existing, valid := ListStoreString(d.store, iter, discoverColPath)
		if valid && fs.CaseEqualFold(existing, path) {
			return true
		}

		ok = d.store.IterNext(iter)
	}

	return false
}

func (d *DiscoverSourcesDialog) onAddFiles() {
	files, ok := SelectFilesDialog(d.parent, "Add Files", d.firstFolder())
	if !ok {
		return
	}

	for _, file := range files {
		d.appendRow(file, false)
	}
}

func (d *DiscoverSourcesDialog) onAddFolder() {
	folder, ok := SelectDirectoryDialog(d.parent, "Add Folder", d.firstFolder())
	if !ok {
		return
	}

	d.appendRow(folder, true)
}

// onPathsDropped adds every dropped file or directory to the sources table; appendRow dedupes.
func (d *DiscoverSourcesDialog) onPathsDropped(paths []string) {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("discover sources: skipping dropped path")

			continue
		}

		d.appendRow(path, info.IsDir())
	}
}

// onRemove deletes every selected row; indices are collected first and removed in descending
// order so earlier removals never invalidate a later row's path.
func (d *DiscoverSourcesDialog) onRemove() {
	rows := d.selection.GetSelectedRows(d.store)

	var indices []int

	for l := rows; l != nil; l = l.Next() {
		path, ok := l.Data().(*gtk.TreePath)
		if !ok {
			continue
		}

		if idx := path.GetIndices(); len(idx) > 0 {
			indices = append(indices, idx[0])
		}
	}

	sort.Sort(sort.Reverse(sort.IntSlice(indices)))

	for _, idx := range indices {
		iter, err := d.store.GetIterFromString(strconv.Itoa(idx))
		if err != nil {
			continue
		}

		d.store.Remove(iter)
	}

	d.updateUI()
}

// firstFolder seeds the pickers at the parent directory of the first source, or its own
// path when it is a directory; "" when the list is empty.
func (d *DiscoverSourcesDialog) firstFolder() string {
	iter, ok := d.store.GetIterFirst()
	if !ok {
		return ""
	}

	path, valid := ListStoreString(d.store, iter, discoverColPath)
	if !valid || path == "" {
		return ""
	}

	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}

	dir, _ := SplitPath(path)

	return dir
}

func (d *DiscoverSourcesDialog) updateUI() {
	count := d.store.IterNChildren(nil)

	d.okButton.SetSensitive(count > 0)

	selected := d.selection.GetSelectedRows(d.store)
	d.removeButton.SetSensitive(selected != nil)
}

// selectedMode reports the radio group's current selection.
func (d *DiscoverSourcesDialog) selectedMode() embedded.FilterMode {
	switch {
	case d.radioWith.GetActive():
		return embedded.FilterWithChecksum
	case d.radioWithout.GetActive():
		return embedded.FilterWithoutChecksum
	default:
		return embedded.FilterAll
	}
}

// Run displays the dialog modally and returns the chosen options; the bool is true on OK.
func (d *DiscoverSourcesDialog) Run() (DiscoverOptions, bool) {
	d.dialog.ShowAll()

	if d.dialog.Run() != gtk.RESPONSE_OK {
		return DiscoverOptions{}, false
	}

	return DiscoverOptions{
		Sources:             d.collectSources(),
		Filter:              d.selectedMode(),
		FollowSymbolicLinks: d.chkFollowSymlinks.GetActive(),
		SkipHidden:          d.chkSkipHidden.GetActive(),
	}, true
}

// Destroy releases the dialog's resources.
func (d *DiscoverSourcesDialog) Destroy() {
	d.dialog.Destroy()
}

// GetSize returns the current dialog width and height; must be called before Destroy.
func (d *DiscoverSourcesDialog) GetSize() (int, int) {
	return d.dialog.GetSize()
}

func (d *DiscoverSourcesDialog) collectSources() []DiscoverSource {
	sources := make([]DiscoverSource, 0)

	iter, ok := d.store.GetIterFirst()
	for ok {
		path, validPath := ListStoreString(d.store, iter, discoverColPath)
		if !validPath {
			MustWidget("ListStore", "DiscoverSourcesDialog.collectSources:path", fmt.Errorf("invalid string at column %d", discoverColPath))
		}

		icon, _ := ListStoreString(d.store, iter, discoverColIcon)

		sources = append(sources, DiscoverSource{Path: path, IsDir: icon == "folder"})

		ok = d.store.IterNext(iter)
	}

	return sources
}

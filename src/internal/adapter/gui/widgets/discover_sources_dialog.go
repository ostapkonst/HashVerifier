package widgets

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/ostapkonst/HashVerifier/internal/domain/scan"
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
	FollowSymbolicLinks bool
	SkipHidden          bool
	FilesToScan         scan.FilesToScan
	Action              scan.Action
}

// DiscoverSourcesDialog is a modal dialog listing the sources to scan; rows are added with
// the file/folder pickers or by dropping them onto the dialog, and removed explicitly with
// the Remove button.
type DiscoverSourcesDialog struct {
	dialog            *gtk.Dialog
	parent            *gtk.Window
	store             *gtk.ListStore
	selection         *gtk.TreeSelection
	removeButton      *gtk.Button
	okButton          *gtk.Button
	chkFollowSymlinks *gtk.CheckButton
	chkSkipHidden     *gtk.CheckButton
	filesToScan       scan.FilesToScan
	action            scan.Action
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

	// The action area is a child of this box, so the spacing also covers the gap to the dialog buttons.
	contentArea.SetSpacing(8)

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
		store:     store,
		selection: selection,
	}

	d.setupColumns(treeView)

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

	hintLabel.SetMarkup(
		"<small>Drag and drop files or folders here to add them.\n" +
			tableSelectionHint + "</small>",
	)
	hintLabel.SetMarginStart(8)
	hintLabel.SetMarginEnd(8)
	hintLabel.SetHAlign(gtk.ALIGN_START)
	contentArea.PackStart(hintLabel, false, false, 0)

	bottomBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		MustWidget("Box", "NewDiscoverSourcesDialog", err)
	}

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

	followBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	if err != nil {
		MustWidget("Box", "NewDiscoverSourcesDialog:options", err)
	}

	followBox.PackStart(d.chkFollowSymlinks, false, false, 0)
	followBox.PackStart(d.chkSkipHidden, false, false, 0)

	scanModeBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	if err != nil {
		MustWidget("Box", "NewDiscoverSourcesDialog:scanMode", err)
	}

	scanAllBtn, err := gtk.RadioButtonNewWithLabel(nil, "All files")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:scanAll", err)
	}

	scanWithChecksumBtn, err := gtk.RadioButtonNewWithLabelFromWidget(scanAllBtn, "With checksum")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:scanWithChecksum", err)
	}

	scanWithoutChecksumBtn, err := gtk.RadioButtonNewWithLabelFromWidget(scanAllBtn, "Without checksum")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:scanWithoutChecksum", err)
	}

	switch current.FilesToScan {
	case scan.FilesToScanAll:
		scanAllBtn.SetActive(true)
	case scan.FilesToScanWithoutChecksum:
		scanWithoutChecksumBtn.SetActive(true)
	default:
		scanWithChecksumBtn.SetActive(true)
	}

	scanModeBox.PackStart(scanAllBtn, false, false, 0)
	scanModeBox.PackStart(scanWithChecksumBtn, false, false, 0)
	scanModeBox.PackStart(scanWithoutChecksumBtn, false, false, 0)

	// The scan scope applies to every listed source and to the contents of every folder.
	filesToScanFrame, err := newSectionFrame("Files to scan")
	if err != nil {
		MustWidget("Frame", "NewDiscoverSourcesDialog:filesToScanFrame", err)
	}

	filesToScanFrame.Add(scanModeBox)
	contentArea.PackStart(filesToScanFrame, false, false, 0)

	actionBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 4)
	if err != nil {
		MustWidget("Box", "NewDiscoverSourcesDialog:action", err)
	}

	reportOnlyBtn, err := gtk.RadioButtonNewWithLabel(nil, "Report only")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:reportOnly", err)
	}

	addChecksumBtn, err := gtk.RadioButtonNewWithLabelFromWidget(reportOnlyBtn, "Add missing")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:addChecksum", err)
	}

	addOrFixChecksumBtn, err := gtk.RadioButtonNewWithLabelFromWidget(reportOnlyBtn, "Add or fix")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:addOrFixChecksum", err)
	}

	removeChecksumBtn, err := gtk.RadioButtonNewWithLabelFromWidget(reportOnlyBtn, "Remove")
	if err != nil {
		MustWidget("RadioButton", "NewDiscoverSourcesDialog:removeChecksum", err)
	}

	switch current.Action {
	case scan.ActionAddMissing:
		addChecksumBtn.SetActive(true)
	case scan.ActionAddOrFix:
		addOrFixChecksumBtn.SetActive(true)
	case scan.ActionRemove:
		removeChecksumBtn.SetActive(true)
	default:
		reportOnlyBtn.SetActive(true)
	}

	actionBox.PackStart(reportOnlyBtn, false, false, 0)
	actionBox.PackStart(addChecksumBtn, false, false, 0)
	actionBox.PackStart(addOrFixChecksumBtn, false, false, 0)
	actionBox.PackStart(removeChecksumBtn, false, false, 0)

	actionFrame, err := newSectionFrame("Checksum actions")
	if err != nil {
		MustWidget("Frame", "NewDiscoverSourcesDialog:actionFrame", err)
	}

	actionFrame.Add(actionBox)
	contentArea.PackStart(actionFrame, false, false, 0)

	// Follow symlinks and Skip hidden govern the contents of scanned folders, not the listed
	// sources themselves, so they are the least prominent setting and sit below the primary choices.
	folderContentsFrame, err := newSectionFrame("Folder contents")
	if err != nil {
		MustWidget("Frame", "NewDiscoverSourcesDialog:folderContentsFrame", err)
	}

	folderContentsFrame.Add(followBox)
	contentArea.PackStart(folderContentsFrame, false, false, 0)

	previewLabel, err := gtk.LabelNew("")
	if err != nil {
		MustWidget("Label", "NewDiscoverSourcesDialog:previewLabel", err)
	}

	// One line per state: the constant-size content box cannot wrap a label correctly, so the
	// texts stay short enough to fit the full width.
	previewLabel.SetXAlign(0)

	previewFrame, err := newSectionFrame("Preview")
	if err != nil {
		MustWidget("Frame", "NewDiscoverSourcesDialog:previewFrame", err)
	}

	previewFrame.Add(previewLabel)
	contentArea.PackStart(previewFrame, false, false, 0)

	updatePreview := func() {
		scope := scan.FilesToScanAll

		switch {
		case scanWithChecksumBtn.GetActive():
			scope = scan.FilesToScanWithChecksum
		case scanWithoutChecksumBtn.GetActive():
			scope = scan.FilesToScanWithoutChecksum
		}

		action := scan.ActionReportOnly

		switch {
		case addChecksumBtn.GetActive():
			action = scan.ActionAddMissing
		case addOrFixChecksumBtn.GetActive():
			action = scan.ActionAddOrFix
		case removeChecksumBtn.GetActive():
			action = scan.ActionRemove
		}

		d.filesToScan = scope
		d.action = action

		previewLabel.SetMarkup("<small>" + previewText(scope, action) + "</small>")
	}

	for _, btn := range []*gtk.RadioButton{
		scanAllBtn, scanWithChecksumBtn, scanWithoutChecksumBtn,
		reportOnlyBtn, addChecksumBtn, addOrFixChecksumBtn, removeChecksumBtn,
	} {
		btn.Connect("toggled", updatePreview)
	}

	updatePreview()

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

func (d *DiscoverSourcesDialog) setupColumns(treeView *gtk.TreeView) {
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
	key := fs.PathKey(path)

	iter, ok := d.store.GetIterFirst()
	for ok {
		existing, valid := ListStoreString(d.store, iter, discoverColPath)
		if valid && fs.PathKey(existing) == key {
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
		d.appendRow(file, isDirPath(file))
	}
}

func (d *DiscoverSourcesDialog) onAddFolder() {
	folder, ok := SelectDirectoryDialog(d.parent, "Add Folder", d.firstFolder())
	if !ok {
		return
	}

	d.appendRow(folder, isDirPath(folder))
}

// onPathsDropped adds every dropped file or directory to the sources table; appendRow dedupes.
func (d *DiscoverSourcesDialog) onPathsDropped(paths []string) {
	for _, path := range paths {
		path = filepath.Clean(path)

		d.appendRow(path, isDirPath(path))
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

	d.okButton.SetLabel(fmt.Sprintf("Discover %d items", count))

	selected := d.selection.GetSelectedRows(d.store)
	d.removeButton.SetSensitive(selected != nil)
}

// Run displays the dialog modally, hides it once the run returns, and reports the chosen
// options; the bool is true on OK.
func (d *DiscoverSourcesDialog) Run() (DiscoverOptions, bool) {
	d.dialog.ShowAll()

	response := d.dialog.Run()
	// gtk_dialog_run leaves the window mapped; hide it so a subsequent message dialog is the
	// only one on screen and no stale modal keeps its grab.
	d.dialog.Hide()

	if response != gtk.RESPONSE_OK {
		return DiscoverOptions{}, false
	}

	return DiscoverOptions{
		Sources:             d.collectSources(),
		FollowSymbolicLinks: d.chkFollowSymlinks.GetActive(),
		SkipHidden:          d.chkSkipHidden.GetActive(),
		FilesToScan:         d.filesToScan,
		Action:              d.action,
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

		sources = append(sources, DiscoverSource{Path: path, IsDir: isDirPath(path)})

		ok = d.store.IterNext(iter)
	}

	return sources
}

// newSectionFrame builds a section whose bold title sits above its content, matching the
// "Summary" sections of the main window: no shadow, left-aligned label.
func newSectionFrame(title string) (*gtk.Frame, error) {
	frame, err := gtk.FrameNew("")
	if err != nil {
		return nil, err
	}

	label, err := gtk.LabelNew("")
	if err != nil {
		return nil, err
	}

	// title is a plain literal, so it needs no markup escaping.
	label.SetMarkup("<b>" + title + "</b>")
	label.SetMarginBottom(4)
	frame.SetLabelWidget(label)
	frame.SetShadowType(gtk.SHADOW_NONE)
	frame.SetLabelAlign(0, 0.5)
	frame.SetMarginStart(8)
	frame.SetMarginEnd(8)

	return frame, nil
}

// previewText returns the one-line description of one scan-scope and action combination; the
// checksum examples use the same generic placeholder as the rest of the UI.
func previewText(scope scan.FilesToScan, action scan.Action) string {
	switch scope {
	case scan.FilesToScanAll:
		switch action {
		case scan.ActionReportOnly:
			return "Verifies files with a checksum; files without one are skipped."
		case scan.ActionAddMissing:
			return "Adds checksums where missing; existing mismatches stay."
		case scan.ActionAddOrFix:
			return "Adds missing checksums and fixes mismatches."
		case scan.ActionRemove:
			return "Removes checksums; files without one are skipped."
		}
	case scan.FilesToScanWithChecksum:
		switch action {
		case scan.ActionReportOnly:
			return "Verifies each checksum; files without one are skipped."
		case scan.ActionAddMissing:
			return "Only verifies; existing mismatches are not fixed."
		case scan.ActionAddOrFix:
			return "Fixes mismatched checksums; correct names stay."
		case scan.ActionRemove:
			return "Removes each checksum: clip [1a2b3c4d].mkv → clip.mkv."
		}
	case scan.FilesToScanWithoutChecksum:
		switch action {
		case scan.ActionReportOnly:
			return "Nothing to verify in these names; all are skipped."
		case scan.ActionAddMissing:
			return "Adds a checksum to every name: clip.mkv → clip [1a2b3c4d].mkv."
		case scan.ActionAddOrFix:
			return "Adds a checksum to every name — nothing to fix."
		case scan.ActionRemove:
			return "Nothing to remove in these names; all are skipped."
		}
	}

	return ""
}

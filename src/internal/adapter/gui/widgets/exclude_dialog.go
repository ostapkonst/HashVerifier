package widgets

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
)

const (
	excludeColChecked  = 0 // gboolean — checkbox state (true = included)
	excludeColName     = 1 // gchararray — display name (basename)
	excludeColRule     = 2 // gchararray — canonical rule string (exclude.Rule.String())
	excludeColIconName = 3 // gchararray — freedesktop icon name (folder/text-x-generic)
)

// ExcludeDialog is a modal dialog listing top-level entries of inputDir with checkboxes;
// the OK label reflects the live excluded count.
type ExcludeDialog struct {
	dialog          *gtk.Dialog
	treeView        *gtk.TreeView
	store           *gtk.ListStore
	okButton        *gtk.Button
	inputDir        string
	outputFile      string
	skipHidden      bool
	lastClickedPath *gtk.TreePath
}

// NewExcludeDialog builds the modal exclude-list dialog: entries matching existing rules start unchecked.
// When skipHidden is set, hidden top-level entries are omitted, mirroring the generate run's skip-hidden filter.
// Panics on GTK widget construction failures (fail-fast pattern, consistent with gtk_getters); an error
// is returned when the source directory cannot be listed, since that is a recoverable user-data failure
// and must not abort the whole application.
func NewExcludeDialog(parent *gtk.Window, title, inputDir, outputFile string, existing []exclude.Rule, skipHidden bool, width, height int) (*ExcludeDialog, error) {
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

	dialog.SetDefaultSize(500, 620)

	if width > 0 && height > 0 {
		dialog.Resize(width, height)
	}

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		MustWidget("ContentArea", "NewExcludeDialog", err)
	}

	// The action area is a child of this box, so the spacing also covers the gap to the dialog buttons.
	contentArea.SetSpacing(8)

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

	treeView.SetEnableSearch(false)
	treeView.SetSearchColumn(-1)
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
		skipHidden: skipHidden,
	}

	d.setupColumns(treeView)

	treeView.Connect("button-press-event", d.onButtonPress)

	scrolledWin, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		MustWidget("ScrolledWindow", "NewExcludeDialog", err)
	}

	scrolledWin.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWin.Add(treeView)

	contentArea.PackStart(scrolledWin, true, true, 0)

	hintLabel, err := gtk.LabelNew("")
	if err != nil {
		MustWidget("Label", "NewExcludeDialog", err)
	}

	hintLabel.SetMarkup("<small>" + tableSelectionHint + "</small>")
	hintLabel.SetMarginStart(8)
	hintLabel.SetMarginEnd(8)
	hintLabel.SetHAlign(gtk.ALIGN_START)
	contentArea.PackStart(hintLabel, false, false, 0)

	bottomBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 8)
	if err != nil {
		MustWidget("Box", "NewExcludeDialog", err)
	}

	bottomBox.SetMarginStart(8)
	bottomBox.SetMarginEnd(8)

	excludeAllBtn, err := gtk.ButtonNewWithLabel("Exclude All")
	if err != nil {
		MustWidget("Button", "NewExcludeDialog:excludeAll", err)
	}

	excludeAllBtn.Connect("clicked", func() { d.setAllChecked(false) })
	bottomBox.PackStart(excludeAllBtn, false, false, 0)

	keepAllBtn, err := gtk.ButtonNewWithLabel("Keep All")
	if err != nil {
		MustWidget("Button", "NewExcludeDialog:keepAll", err)
	}

	keepAllBtn.Connect("clicked", func() { d.setAllChecked(true) })
	bottomBox.PackStart(keepAllBtn, false, false, 0)

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

	if err := d.buildList(existing); err != nil {
		return nil, err
	}

	d.updateExcludedUI()

	success = true

	return d, nil
}

func (d *ExcludeDialog) setupColumns(treeView *gtk.TreeView) {
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
}

func (d *ExcludeDialog) buildList(existing []exclude.Rule) error {
	entries, err := os.ReadDir(d.inputDir)
	if err != nil {
		return fmt.Errorf("read source directory %s: %w", d.inputDir, err)
	}

	sortDirEntriesByDirFirst(d.inputDir, entries)

	var hiddenChecker *fs.HiddenChecker
	if d.skipHidden {
		hiddenChecker = fs.NewHiddenChecker(d.inputDir)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(d.inputDir, entry.Name())

		if hiddenChecker.IsHidden(entry.Name()) {
			continue
		}

		// outputFile may be empty when the caller hasn't picked a target yet; the guard avoids
		// comparing against the process CWD, which is what filepath.Abs("") yields.
		if d.outputFile != "" {
			isOutput, err := fs.SameFile(fullPath, d.outputFile)
			if err != nil {
				log.Warn().Err(err).Str("path", fullPath).Msg("SameFile failed; entry shown to user")
			} else if isOutput {
				continue
			}
		}

		isDir := entryIsDir(d.inputDir, entry)

		// The Rule is just the entry's name — the matcher treats "build" the same as "build/"
		// because one path component cannot be both a file and a directory on any supported FS.
		rule, err := exclude.ParseRule(entry.Name())
		if err != nil {
			log.Warn().Err(err).Str("name", entry.Name()).Msg("exclude dialog: skipping unrepresentable entry")
			continue
		}

		checked := true

		for _, prev := range existing {
			if fs.CaseEqualFold(prev.String(), rule.String()) {
				checked = false
				break
			}
		}

		iter := d.store.Append()

		if err := d.store.SetValue(iter, excludeColChecked, checked); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:checked", err)
		}

		if err := d.store.SetValue(iter, excludeColName, entry.Name()); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:name", err)
		}

		if err := d.store.SetValue(iter, excludeColRule, rule.String()); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:rule", err)
		}

		iconName := "text-x-generic"
		if isDir {
			iconName = "folder"
		}

		if err := d.store.SetValue(iter, excludeColIconName, iconName); err != nil {
			MustWidget("ListStore", "ExcludeDialog.buildList:icon", err)
		}
	}

	return nil
}

// entryIsDir reports whether entry is a directory, following symlinks so a symlink-to-dir
// is rendered (and excluded) as a directory, matching walker semantics with follow-symbolic-links=true.
func entryIsDir(inputDir string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}

	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}

	return isDirPath(filepath.Join(inputDir, entry.Name()))
}

func (d *ExcludeDialog) onToggle(iter *gtk.TreeIter) {
	checked, err := d.boolValue(iter, excludeColChecked)
	if err != nil {
		MustWidget("ListStore", "ExcludeDialog.onToggle:GetValue", err)
	}

	if err := d.store.SetValue(iter, excludeColChecked, !checked); err != nil {
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
	count := len(d.collectExcludedRules())
	d.okButton.SetLabel(fmt.Sprintf("Exclude %d items", count))
}

// Run displays the dialog modally, hides it once the run returns, and reports the excluded Rules
// (with directory semantics kept); the bool is true on OK.
func (d *ExcludeDialog) Run() ([]exclude.Rule, bool) {
	d.dialog.ShowAll()

	response := d.dialog.Run()
	// gtk_dialog_run leaves the window mapped; hide it so a subsequent message dialog is the
	// only one on screen and no stale modal keeps its grab.
	d.dialog.Hide()

	if response != gtk.RESPONSE_OK {
		return nil, false
	}

	return d.collectExcludedRules(), true
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

func (d *ExcludeDialog) ruleValue(iter *gtk.TreeIter) (exclude.Rule, error) {
	val, err := d.store.GetValue(iter, excludeColRule)
	if err != nil {
		return exclude.Rule{}, fmt.Errorf("read rule column: %w", err)
	}

	v, err := val.GoValue()
	if err != nil {
		return exclude.Rule{}, fmt.Errorf("extract Go value from rule column: %w", err)
	}

	s, ok := v.(string)
	if !ok {
		return exclude.Rule{}, fmt.Errorf("expected string, got %T", v)
	}

	rule, err := exclude.ParseRule(s)
	if err != nil {
		return exclude.Rule{}, fmt.Errorf("re-parse rule %q: %w", s, err)
	}

	return rule, nil
}

// collectExcludedRules reads the store and returns Rules for every unchecked row.
// Per-row failures panic via MustWidget semantics: returning partial data here would silently
// drop user intent.
func (d *ExcludeDialog) collectExcludedRules() []exclude.Rule {
	var rules []exclude.Rule

	iter, ok := d.store.GetIterFirst()
	for ok {
		checked, err := d.boolValue(iter, excludeColChecked)
		if err != nil {
			MustWidget("ListStore", "ExcludeDialog.collectExcludedRules:checked", err)
		}

		if !checked {
			rule, err := d.ruleValue(iter)
			if err != nil {
				MustWidget("ListStore", "ExcludeDialog.collectExcludedRules:rule", err)
			}

			rules = append(rules, rule)
		}

		ok = d.store.IterNext(iter)
	}

	return rules
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

func sortDirEntriesByDirFirst(inputDir string, entries []os.DirEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		iDir := entryIsDir(inputDir, entries[i])
		jDir := entryIsDir(inputDir, entries[j])

		if iDir != jDir {
			return iDir
		}

		return entries[i].Name() < entries[j].Name()
	})
}

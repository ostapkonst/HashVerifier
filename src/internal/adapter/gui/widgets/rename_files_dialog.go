package widgets

import (
	"fmt"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
)

const (
	renameColDir     = 0 // gchararray — containing directory (display)
	renameColCurrent = 1 // gchararray — current base name
	renameColNew     = 2 // gchararray — base name after the selected mode

	// renamePreviewLimit caps the previewed rows; every changed file is still renamed.
	renamePreviewLimit = 100
)

// RenameOptions is the rename dialog's result: the planned actions and the mode that produced them.
type RenameOptions struct {
	Actions []embedded.RenameAction
	Mode    embedded.RenameMode
}

// RenameFilesDialog is a modal preview of the renames planned for the last discovery run; a
// radio group selects the rename mode and the table previews up to renamePreviewLimit changed files.
type RenameFilesDialog struct {
	dialog       *gtk.Dialog
	store        *gtk.ListStore
	treeView     *gtk.TreeView
	previewLabel *gtk.Label
	okButton     *gtk.Button
	radioAdd     *gtk.RadioButton
	radioStrip   *gtk.RadioButton
	radioFix     *gtk.RadioButton
	candidates   []embedded.RenameCandidate
}

// NewRenameFilesDialog builds the modal rename dialog for candidates (files from the last
// discovery run). Panics on GTK widget construction failures (fail-fast, consistent with gtk_getters).
func NewRenameFilesDialog(
	parent *gtk.Window,
	title string,
	candidates []embedded.RenameCandidate,
	mode embedded.RenameMode,
	width, height int,
) *RenameFilesDialog {
	dialog, err := gtk.DialogNewWithButtons(
		title,
		parent,
		gtk.DIALOG_MODAL,
		[]interface{}{"_Cancel", gtk.RESPONSE_CANCEL},
		[]interface{}{"Rename", gtk.RESPONSE_OK},
	)
	if err != nil {
		MustWidget("Dialog", "NewRenameFilesDialog", err)
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
		MustWidget("ContentArea", "NewRenameFilesDialog", err)
	}

	store, err := gtk.ListStoreNew(glib.TYPE_STRING, glib.TYPE_STRING, glib.TYPE_STRING)
	if err != nil {
		MustWidget("ListStore", "NewRenameFilesDialog", err)
	}

	d := &RenameFilesDialog{
		dialog:     dialog,
		store:      store,
		candidates: candidates,
	}

	treeView, err := gtk.TreeViewNewWithModel(store)
	if err != nil {
		MustWidget("TreeView", "NewRenameFilesDialog", err)
	}

	treeView.SetEnableSearch(false)
	treeView.SetSearchColumn(-1)

	d.treeView = treeView

	if err := d.setupColumns(treeView); err != nil {
		MustWidget("RenameFilesDialog.Columns", "NewRenameFilesDialog", err)
	}

	scrolledWin, err := gtk.ScrolledWindowNew(nil, nil)
	if err != nil {
		MustWidget("ScrolledWindow", "NewRenameFilesDialog", err)
	}

	scrolledWin.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
	scrolledWin.Add(treeView)

	contentArea.PackStart(scrolledWin, true, true, 0)

	d.previewLabel, err = gtk.LabelNew("")
	if err != nil {
		MustWidget("Label", "NewRenameFilesDialog:preview", err)
	}

	d.previewLabel.SetMarginTop(8)
	d.previewLabel.SetMarginBottom(8)
	d.previewLabel.SetMarginStart(8)
	d.previewLabel.SetMarginEnd(8)
	d.previewLabel.SetHAlign(gtk.ALIGN_START)
	contentArea.PackStart(d.previewLabel, false, false, 0)

	d.radioAdd, err = gtk.RadioButtonNewWithLabel(nil, "Add CRC-32 to names")
	if err != nil {
		MustWidget("RadioButton", "NewRenameFilesDialog:add", err)
	}

	d.radioStrip, err = gtk.RadioButtonNewWithLabelFromWidget(d.radioAdd, "Remove CRC-32 from names")
	if err != nil {
		MustWidget("RadioButton", "NewRenameFilesDialog:strip", err)
	}

	d.radioFix, err = gtk.RadioButtonNewWithLabelFromWidget(d.radioAdd, "Update CRC-32 in names")
	if err != nil {
		MustWidget("RadioButton", "NewRenameFilesDialog:fix", err)
	}

	switch mode {
	case embedded.RenameRemove:
		d.radioStrip.SetActive(true)
	case embedded.RenameUpdate:
		d.radioFix.SetActive(true)
	default:
		d.radioAdd.SetActive(true)
	}

	d.radioAdd.Connect("toggled", d.refresh)
	d.radioStrip.Connect("toggled", d.refresh)
	d.radioFix.Connect("toggled", d.refresh)

	modeBox, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 4)
	if err != nil {
		MustWidget("Box", "NewRenameFilesDialog:mode", err)
	}

	modeBox.SetMarginTop(8)
	modeBox.SetMarginBottom(8)
	modeBox.SetMarginStart(8)
	modeBox.SetMarginEnd(8)
	modeBox.PackStart(d.radioAdd, false, false, 0)
	modeBox.PackStart(d.radioStrip, false, false, 0)
	modeBox.PackStart(d.radioFix, false, false, 0)

	contentArea.PackStart(modeBox, false, false, 0)

	okBtn, err := dialog.GetWidgetForResponse(gtk.RESPONSE_OK)
	if err != nil {
		MustWidget("DialogResponse", "NewRenameFilesDialog", err)
	}

	button, ok := okBtn.(*gtk.Button)
	if !ok {
		MustWidget("Button", "NewRenameFilesDialog:typeAssert", fmt.Errorf("OK widget is not a Button"))
	}

	d.okButton = button

	d.refresh()

	success = true

	return d
}

func (d *RenameFilesDialog) setupColumns(treeView *gtk.TreeView) error {
	columns := []struct {
		title string
		col   int
	}{
		{"Directory", renameColDir},
		{"Current Name", renameColCurrent},
		{"New Name", renameColNew},
	}

	for _, spec := range columns {
		cell, err := gtk.CellRendererTextNew()
		if err != nil {
			MustWidget("CellRendererText", "RenameFilesDialog.setupColumns", err)
		}

		col, err := gtk.TreeViewColumnNew()
		if err != nil {
			MustWidget("TreeViewColumn", "RenameFilesDialog.setupColumns", err)
		}

		col.SetTitle(spec.title)
		col.SetResizable(true)
		col.PackStart(cell, true)
		col.AddAttribute(cell, "text", spec.col)
		treeView.AppendColumn(col)
	}

	return nil
}

// refresh rebuilds the preview table with the candidates that change under the selected mode,
// capped at renamePreviewLimit rows, and updates the footnote and OK label/sensitivity to the full count.
func (d *RenameFilesDialog) refresh() {
	mode := d.selectedMode()

	d.store.Clear()
	ResetColumnWidths(d.treeView)

	total := 0
	shown := 0

	for _, candidate := range d.candidates {
		base := filepath.Base(candidate.Path)

		newName, changed := embedded.PlanRename(base, candidate.Checksum, mode)
		if !changed {
			continue
		}

		total++

		// The table is a preview; every changed file is still renamed (see collectActions).
		if shown >= renamePreviewLimit {
			continue
		}

		shown++

		iter := d.store.Append()

		values := []string{filepath.Dir(candidate.Path), base, newName}

		for col, value := range values {
			if err := d.store.SetValue(iter, col, value); err != nil {
				MustWidget("ListStore", "RenameFilesDialog.refresh", err)
			}
		}
	}

	d.previewLabel.SetMarkup(fmt.Sprintf("<small>Showing %d of %d records for preview</small>", shown, total))
	d.okButton.SetLabel(fmt.Sprintf("Rename %d files", total))
	d.okButton.SetSensitive(total > 0)
}

func (d *RenameFilesDialog) selectedMode() embedded.RenameMode {
	switch {
	case d.radioStrip.GetActive():
		return embedded.RenameRemove
	case d.radioFix.GetActive():
		return embedded.RenameUpdate
	default:
		return embedded.RenameAdd
	}
}

// Run displays the dialog modally and returns the chosen options; the bool is true on OK.
func (d *RenameFilesDialog) Run() (RenameOptions, bool) {
	d.dialog.ShowAll()

	if d.dialog.Run() != gtk.RESPONSE_OK {
		return RenameOptions{}, false
	}

	return RenameOptions{
		Actions: d.collectActions(),
		Mode:    d.selectedMode(),
	}, true
}

// Destroy releases the dialog's resources.
func (d *RenameFilesDialog) Destroy() {
	d.dialog.Destroy()
}

// GetSize returns the current dialog width and height; must be called before Destroy.
func (d *RenameFilesDialog) GetSize() (int, int) {
	return d.dialog.GetSize()
}

// collectActions plans a rename for every changed candidate (not just the previewed rows), so the
// 100-row table limit never truncates the work.
func (d *RenameFilesDialog) collectActions() []embedded.RenameAction {
	mode := d.selectedMode()

	actions := make([]embedded.RenameAction, 0)

	for _, candidate := range d.candidates {
		base := filepath.Base(candidate.Path)

		newName, changed := embedded.PlanRename(base, candidate.Checksum, mode)
		if !changed {
			continue
		}

		actions = append(actions, embedded.RenameAction{
			From: candidate.Path,
			To:   filepath.Join(filepath.Dir(candidate.Path), newName),
		})
	}

	return actions
}

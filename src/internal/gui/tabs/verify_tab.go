package tabs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/action"
	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/internal/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/settings"
	"github.com/ostapkonst/HashVerifier/utils/unwrap"
)

type VerifyTab struct {
	*TabBase
	entryChecksum       *gtk.Entry
	btnStart            *gtk.Button
	btnStop             *gtk.Button
	btnBrowseChk        *gtk.Button
	treeValidate        *gtk.TreeView
	listStore           *gtk.ListStore
	chkBoxVerifyOnOpen  *gtk.CheckButton
	contextMenuProvider *widgets.ContextMenuProvider
	progressTracker     *ProgressTracker
	cmbTxtAlgorithm     *gtk.ComboBoxText
	labelMatchV         *gtk.Label
	labelMismatchV      *gtk.Label
	labelUnreadableV    *gtk.Label
	labelPendingV       *gtk.Label
	labelSpeedV         *gtk.Label
}

func NewVerifyTab(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings) *VerifyTab {
	tab := &VerifyTab{
		TabBase: NewTabBase(ctx, builder, window, settings, NewVerifyColumnConfig()),
	}
	tab.getWidgets()
	tab.getLabels()
	tab.progressTracker = NewProgressTracker(
		tab.Builder,
		"grid_val_progress",
		"progress_val_total",
		"progress_val_curr_file",
		"label_val_curr_file_value",
	)
	tab.contextMenuProvider = widgets.NewContextMenuProvider(tab.treeValidate, tab.listStore)
	tab.applySettingsToUI()
	tab.setStartState()
	tab.setupHandlers()

	return tab
}

func (t *VerifyTab) Fill(path string) error {
	if t.IsBusy() {
		return ErrTabBusy
	}

	t.entryChecksum.SetText(path)
	t.onEntryChecksumChanged(true, t.onStart)

	return nil
}

func (t *VerifyTab) getWidgets() {
	t.entryChecksum = widgets.GetEntry(t.Builder, "entry_val_checksum")
	t.btnStart = widgets.GetButton(t.Builder, "btn_start_validate")
	t.btnStop = widgets.GetButton(t.Builder, "btn_stop_validate")
	t.btnBrowseChk = widgets.GetButton(t.Builder, "btn_browse_val_checksum")
	t.treeValidate = widgets.GetTreeView(t.Builder, "tree_validate")
	t.listStore = widgets.GetListStore(t.Builder, "liststore_validate")
	t.chkBoxVerifyOnOpen = widgets.GetCheckButton(t.Builder, "chk_val_verify_on_open")
	t.cmbTxtAlgorithm = widgets.GetComboBoxText(t.Builder, "cmb_val_algorithm")
}

func (t *VerifyTab) getLabels() {
	t.labelMatchV = widgets.GetLabel(t.Builder, "label_val_match_value")
	t.labelMismatchV = widgets.GetLabel(t.Builder, "label_val_mismatch_value")
	t.labelUnreadableV = widgets.GetLabel(t.Builder, "label_val_unreadable_value")
	t.labelPendingV = widgets.GetLabel(t.Builder, "label_val_pending_value")
	t.labelSpeedV = widgets.GetLabel(t.Builder, "label_val_speed_value")
}

func (t *VerifyTab) setupHandlers() {
	t.btnBrowseChk.Connect("clicked", func() {
		path, _ := t.entryChecksum.GetText()
		if file, ok := widgets.OpenFileDialog(t.Window, "Select Checksum File", path); ok {
			t.entryChecksum.SetText(file)
			t.onEntryChecksumChanged(true, t.onStart)
		}
	})
	t.entryChecksum.Connect("changed", func() {
		t.onEntryChecksumChanged(true, nil)
	})
	t.btnStart.Connect("clicked", t.onStart)
	t.btnStop.Connect("clicked", t.onStop)
	t.chkBoxVerifyOnOpen.Connect("toggled", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save verify settings", err)
		}
	})
	t.treeValidate.Connect("columns-changed", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save verify settings", err)
		}
	})
	t.setupContextMenu()
	t.SetupColumnHandlers(t.treeValidate, func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save verify settings", err)
		}
	})
}

func (t *VerifyTab) onStart() {
	checksumFile, _ := t.entryChecksum.GetText()
	checksumFile = filepath.Clean(checksumFile)
	lastStats := checksum.NewVerifierStats()
	currentIdx := int64(0)

	t.activateStopState()

	ctx, cancel := context.WithCancel(t.Ctx)
	t.Cancel = cancel

	cfg := action.VerifyStreamingConfig{
		CheckSumFile: checksumFile,
		Extension:    t.cmbTxtAlgorithm.GetActiveID(),
	}

	results, err := action.VerifyChecksumsStreaming(ctx, cfg)
	if err != nil {
		t.CancelOperation()
		t.setStartState()

		widgets.ShowError(t.Window, "Verification Error", fmt.Sprintf("Failed to start verification: %v", err))

		return
	}

	log.Info().
		Str("checksum_file", checksumFile).
		Msg("Starting verification")

	t.Wg.Add(1)

	var hasError error

	go func() {
		defer t.Wg.Done()

		for res := range results {
			if res.IsProgressUpdate {
				glib.IdleAdd(func() {
					lastStats = res.Stats
					t.updateStats(lastStats)
				})
			}

			if res.Err != nil {
				hasError = res.Err
				break
			}

			if res.IsProgressUpdate {
				continue
			}

			glib.IdleAdd(func() {
				currentIdx += 1
				iter := t.listStore.Append()
				_ = t.listStore.SetValue(iter, 0, currentIdx)
				_ = t.listStore.SetValue(iter, 1, res.Result.Path)
				_ = t.listStore.SetValue(iter, 2, bytesize.New(float64(res.Result.ReadBytes)).String())
				_ = t.listStore.SetValue(iter, 3, res.Result.Status.String())
				_ = t.listStore.SetValue(iter, 4, res.Result.ActualHash)

				_ = t.listStore.SetValue(iter, 5, res.Result.ExpectedHash)
				if res.Result.Err != nil {
					_ = t.listStore.SetValue(iter, 6, unwrap.UnwrapAndNormalize(res.Result.Err))
				}

				_ = t.listStore.SetValue(iter, 7, res.Result.Status.Color())
				_ = t.listStore.SetValue(iter, 8, res.Result.ReadBytes)
				_ = t.listStore.SetValue(iter, 9, res.Result.FullPath)
				_ = t.listStore.SetValue(iter, 10, res.Result.Status.Priority())
				lastStats = res.Stats
				t.updateStats(lastStats)
			})
		}
	}()
	go func() {
		t.Wg.Wait()
		func() {
			if hasError != nil {
				if errors.Is(hasError, context.Canceled) {
					log.Warn().Msg("Verification canceled")
					return
				}

				log.Error().Err(hasError).Msg("Failed to verify checksums")
				glib.IdleAdd(func() {
					widgets.ShowError(t.Window, "Verification Error", fmt.Sprintf("Failed to verify checksums: %v", hasError))
				})

				return
			}

			log.Info().
				Int("matched", lastStats.Matched).
				Int("mismatch", lastStats.Mismatch).
				Int("unreadable", lastStats.Unreadable).
				Int("pending", lastStats.Pending()).
				Int("total_files", lastStats.TotalFiles).
				Msg("Verification stats")
			log.Info().Msg("Verification completed")
		}()
		glib.IdleAdd(func() {
			t.CancelOperation()
			t.setStartState()
		})
	}()
}

func (t *VerifyTab) onStop() {
	t.CancelOperation()
}

func (t *VerifyTab) activateStopState() {
	lastStats := checksum.NewVerifierStats()

	t.listStore.Clear()
	t.updateStats(lastStats)

	t.btnStart.SetVisible(false)
	t.btnStop.SetVisible(true)
	t.progressTracker.ActivateStopState()
	t.btnBrowseChk.SetSensitive(false)
	t.entryChecksum.SetSensitive(false)
	t.chkBoxVerifyOnOpen.SetSensitive(false)
	t.cmbTxtAlgorithm.SetSensitive(false)
}

func (t *VerifyTab) setStartState() {
	t.btnStart.SetVisible(true)
	t.btnStop.SetVisible(false)
	t.progressTracker.SetStartState()
	t.btnBrowseChk.SetSensitive(true)
	t.entryChecksum.SetSensitive(true)
	t.chkBoxVerifyOnOpen.SetSensitive(true)

	t.onEntryChecksumChanged(false, nil)
}

func (t *VerifyTab) updateStats(stats checksum.VerifierStats) {
	t.labelMatchV.SetText(fmt.Sprintf("%d of %d files", stats.Matched, stats.TotalFiles))
	t.labelMismatchV.SetText(fmt.Sprintf("%d of %d files", stats.Mismatch, stats.TotalFiles))
	t.labelUnreadableV.SetText(fmt.Sprintf("%d of %d files", stats.Unreadable, stats.TotalFiles))
	t.labelPendingV.SetText(fmt.Sprintf("%d of %d files", stats.Pending(), stats.TotalFiles))
	t.labelSpeedV.SetText(bytesize.New(stats.Speed).String() + "/s")
	t.progressTracker.UpdateCurrentFile(stats.CurrentFileOrStatus)
	t.progressTracker.UpdateTotalProgress(stats.TotalProgress())
	t.progressTracker.UpdateFileProgress(stats.FileHashingProgress)
}

func (t *VerifyTab) Wait() {
	t.Wg.Wait()
}

func (t *VerifyTab) applySettingsToUI() {
	t.chkBoxVerifyOnOpen.SetActive(t.Settings.Verify.VerifyOnOpen)
	t.ColumnConfig.ApplyColumnOrder(t.treeValidate, t.Settings.Verify.ColumnOrder)
	t.ApplySortOrder(t.treeValidate, t.Settings.Verify.SortColumn, t.Settings.Verify.SortOrder)
}

func (t *VerifyTab) saveSettings() error {
	if t.Window.InDestruction() {
		return nil
	}

	t.Settings.Verify.VerifyOnOpen = t.chkBoxVerifyOnOpen.GetActive()
	t.Settings.Verify.ColumnOrder = t.ColumnConfig.GetColumnOrder(t.treeValidate)
	sortColumn, sortOrder := t.ColumnConfig.GetSortState(t.treeValidate)

	t.Settings.Verify.SortColumn = sortColumn
	if sortOrder == gtk.SORT_DESCENDING {
		t.Settings.Verify.SortOrder = settings.SortOrderDesc
	} else {
		t.Settings.Verify.SortOrder = settings.SortOrderAsc
	}

	return t.Settings.Save()
}

func (t *VerifyTab) setupContextMenu() {
	columnLabels := []string{"index", "path", "size", "status", "hash", "expected hash", "note"}
	t.contextMenuProvider.CreateMenu(9, columnLabels)
	t.contextMenuProvider.ConnectRightClick(func() {
		t.contextMenuProvider.ShowMenu()
	})
}

func (t *VerifyTab) onEntryChecksumChanged(updateActiveID bool, onStartFunc func()) {
	path, _ := t.entryChecksum.GetText()

	algo, err := checksum.AlgorithmFromExtension(path)
	foundByExt := err == nil

	if !foundByExt {
		algo, err = checksum.AlgorithmFromAllSumsFiles(path)
	}

	if err == nil {
		t.cmbTxtAlgorithm.SetSensitive(!foundByExt)

		if updateActiveID {
			t.cmbTxtAlgorithm.SetActiveID(algo.Extension())
		}

		if onStartFunc != nil && t.chkBoxVerifyOnOpen.GetActive() {
			onStartFunc()
		}

		return
	}

	t.cmbTxtAlgorithm.SetSensitive(true)

	if updateActiveID {
		t.cmbTxtAlgorithm.SetActiveID(".unknown")
	}
}

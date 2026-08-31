// Package verify implements the Verify GUI tab: checksum file picker, algorithm selector, and a streaming table of comparison results.
package verify

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotk3/gotk3/gtk"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/reveal"
	"github.com/ostapkonst/HashVerifier/internal/service/verify"
)

// VerifyTab embeds base.TabBase and adds GTK widgets and callbacks for the Verify use-case.
type VerifyTab struct {
	*base.TabBase
	entryChecksum       *gtk.Entry
	btnStart            *gtk.Button
	btnStop             *gtk.Button
	btnBrowseChk        *gtk.Button
	treeValidate        *gtk.TreeView
	listStore           *gtk.ListStore
	chkBoxVerifyOnOpen  *gtk.CheckButton
	contextMenuProvider *widgets.ContextMenuProvider
	progressTracker     *base.ProgressTracker
	cmbTxtAlgorithm     *gtk.ComboBoxText
	labelMatchV         *gtk.Label
	labelMismatchV      *gtk.Label
	labelUnreadableV    *gtk.Label
	labelPendingV       *gtk.Label
	labelSpeedV         *gtk.Label
}

// NewVerifyTab initializes the Verify tab's widgets, restores persisted settings, and connects every handler.
func NewVerifyTab(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings) *VerifyTab {
	tab := &VerifyTab{
		TabBase: base.NewTabBase(ctx, builder, window, settings, widgets.NewVerifyColumnConfig()),
	}
	tab.getWidgets()
	tab.getLabels()
	tab.progressTracker = base.NewProgressTracker(
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

// Fill sets the checksum-file field from path and auto-starts when VerifyOnOpen is enabled and the algorithm
// resolves from the filename. Returns base.ErrTabBusy if the tab is currently running.
func (t *VerifyTab) Fill(path string) error {
	if t.IsBusy() {
		return base.ErrTabBusy
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
	t.setupContextMenu()
	t.SetupColumnHandlers(t.treeValidate, func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save verify settings", err)
		}
	})
}

func (t *VerifyTab) validateInputs(checksumFile string) bool {
	if checksumFile == "" {
		widgets.ShowError(t.Window, "No Checksum File", "Please select a checksum file.")

		return false
	}

	info, err := os.Stat(checksumFile)
	if err != nil {
		widgets.ShowError(t.Window, "Checksum File Not Found",
			fmt.Sprintf("Checksum file does not exist:\n%s", checksumFile))

		return false
	}

	if info.IsDir() {
		widgets.ShowError(t.Window, "Invalid Checksum Path",
			fmt.Sprintf("Checksum path is a directory:\n%s", checksumFile))

		return false
	}

	if !info.Mode().IsRegular() {
		widgets.ShowError(t.Window, "Invalid Checksum Path",
			fmt.Sprintf("Checksum path is not a regular file:\n%s", checksumFile))

		return false
	}

	algoID := t.cmbTxtAlgorithm.GetActiveID()
	if algoID == "" || algoID == ".unknown" {
		widgets.ShowError(t.Window, "Unknown Algorithm",
			"Cannot determine algorithm from file extension. Please select one manually.")

		return false
	}

	return true
}

func (t *VerifyTab) onStart() {
	checksumFile, _ := t.entryChecksum.GetText()

	if !t.validateInputs(checksumFile) {
		return
	}

	checksumFile = filepath.Clean(checksumFile)

	lastStats := result.NewVerifierStats()
	currentIdx := int64(0)

	t.activateStopState()

	ctx, cancel := context.WithCancel(t.Ctx)
	t.SetCancel(cancel)

	algo, _ := algorithm.AlgorithmFromExtension(t.cmbTxtAlgorithm.GetActiveID())

	cfg := verify.VerifyStreamingConfig{
		ChecksumFile: checksumFile,
		Algorithm:    algo,
	}

	results, err := verify.VerifyChecksumsStreaming(ctx, cfg)
	if err != nil {
		t.CancelOperation()
		t.setStartState()

		widgets.ShowError(t.Window, "Verification Error", fmt.Sprintf("Failed to start verification: %v", err))

		return
	}

	log.Info().
		Str("checksum_file", checksumFile).
		Str("algorithm", cfg.Algorithm.String()).
		Msg("Starting verification")

	t.Wg.Add(1)

	appendRows := func(items []verify.VerifyStreamingResult) {
		widgets.IdleAdd(t.Window, func() {
			for i := range items {
				r := items[i]
				currentIdx += 1

				iter := t.listStore.Append()
				if err := t.listStore.SetValue(iter, 0, currentIdx); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col0", err)
				}

				if err := t.listStore.SetValue(iter, 1, r.Result.Path); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col1", err)
				}

				if err := t.listStore.SetValue(iter, 2, bytesize.New(float64(r.Result.ReadBytes)).String()); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col2", err)
				}

				if err := t.listStore.SetValue(iter, 3, r.Result.Status.String()); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col3", err)
				}

				if err := t.listStore.SetValue(iter, 4, r.Result.ActualHash); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col4", err)
				}

				if err := t.listStore.SetValue(iter, 5, r.Result.ExpectedHash); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col5", err)
				}

				if r.Result.Err != nil {
					if err := t.listStore.SetValue(iter, 6, errs.UnwrapAndNormalize(r.Result.Err)); err != nil {
						widgets.MustWidget("ListStore", "VerifyTab.appendRows:col6", err)
					}
				}

				if err := t.listStore.SetValue(iter, 7, r.Result.Status.Color()); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col7", err)
				}

				if err := t.listStore.SetValue(iter, 8, r.Result.ReadBytes); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col8", err)
				}

				if err := t.listStore.SetValue(iter, 9, r.Result.FullPath); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col9", err)
				}

				if err := t.listStore.SetValue(iter, 10, r.Result.Status.Priority()); err != nil {
					widgets.MustWidget("ListStore", "VerifyTab.appendRows:col10", err)
				}

				lastStats = r.Stats
			}

			t.updateStats(lastStats)
		})
	}

	go func() {
		defer t.Wg.Done()

		widgets.RunStream(results, widgets.StreamBatchConfig[verify.VerifyStreamingResult]{
			FlushSize:     200,
			FlushInterval: 150 * time.Millisecond,
			IsProgress:    func(r verify.VerifyStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r verify.VerifyStreamingResult) error { return r.Err },
			OnProgress: func(r verify.VerifyStreamingResult) {
				widgets.IdleAdd(t.Window, func() {
					lastStats = r.Stats
					t.updateStats(lastStats)
				})
			},
			OnBatch: appendRows,
			OnFinish: func(hasError error) {
				widgets.IdleAdd(t.Window, func() {
					if hasError != nil {
						if errs.IsSoleCancelCause(hasError) {
							log.Warn().Msg("Verification canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to verify checksums")
							widgets.ShowError(t.Window, "Verification Error",
								fmt.Sprintf("Failed to verify checksums: %v", hasError))
						}
					} else {
						log.Info().
							Int("matched", lastStats.Matched).
							Int("mismatch", lastStats.Mismatch).
							Int("unreadable", lastStats.Unreadable).
							Int("pending", lastStats.Pending()).
							Int("total_files", lastStats.TotalFiles).
							Msg("Verification stats")
						log.Info().Msg("Verification completed")
					}

					t.CancelOperation()
					t.setStartState()

					color := result.HashMismatch.Color()
					if hasError == nil && lastStats.Mismatch+lastStats.Unreadable == 0 && lastStats.Pending() == 0 {
						color = result.HashMatched.Color()
					}

					base.SetFinalLabel(
						t.labelMatchV,
						lastStats.Matched, lastStats.TotalFiles,
						color,
					)
				})
			},
		})
	}()
}

func (t *VerifyTab) onStop() {
	t.CancelOperation()
}

func (t *VerifyTab) activateStopState() {
	lastStats := result.NewVerifierStats()

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

func (t *VerifyTab) updateStats(stats result.VerifierStats) {
	t.labelMatchV.SetText(fmt.Sprintf("%d of %d files", stats.Matched, stats.TotalFiles))
	base.SetStatLabel(t.labelMismatchV, stats.Mismatch, stats.TotalFiles, result.HashMismatch.Color())
	base.SetStatLabel(t.labelUnreadableV, stats.Unreadable, stats.TotalFiles, result.Unreadable.Color())
	t.labelPendingV.SetText(fmt.Sprintf("%d of %d files", stats.Pending(), stats.TotalFiles))
	t.labelSpeedV.SetText(bytesize.New(stats.Speed).String() + "/s")
	t.progressTracker.UpdateCurrentFile(stats.CurrentFileOrStatus)
	t.progressTracker.UpdateTotalProgress(stats.TotalProgress())
	t.progressTracker.UpdateFileProgress(stats.FileHashingProgress)
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
	t.contextMenuProvider.CreateMenuWithReveal(9, columnLabels, t.revealSelectedFile)
	t.contextMenuProvider.ConnectRightClick(func() {
		t.contextMenuProvider.ShowMenu()
	})
}

func (t *VerifyTab) revealSelectedFile(fullPath string) {
	go func() {
		if err := reveal.Reveal(t.Ctx, fullPath); err != nil {
			widgets.IdleAdd(t.Window, func() {
				widgets.ShowError(t.Window, "Reveal Error",
					fmt.Sprintf("Failed to open file manager:\n%v", err))
			})

			return
		}
	}()
}

func (t *VerifyTab) onEntryChecksumChanged(updateActiveID bool, onStartFunc func()) {
	path, _ := t.entryChecksum.GetText()

	algo, err := algorithm.AlgorithmFromExtension(path)
	foundByExt := err == nil

	if !foundByExt {
		algo, err = algorithm.AlgorithmFromAllSumsFiles(path)
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

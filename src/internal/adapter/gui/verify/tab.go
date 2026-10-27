// Package verify implements the Verify GUI tab: checksum file picker, algorithm selector,
// and a streaming table of comparison results.
package verify

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
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
	labelTotalV         *gtk.Label
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
// resolves from the filename.
// Returns base.ErrTabBusy if the tab is currently running.
func (t *VerifyTab) Fill(path string) error {
	if t.IsBusy() {
		return base.ErrTabBusy
	}

	t.entryChecksum.SetText(path)
	t.onEntryChecksumChanged(true, t.onStart)

	return nil
}

// FocusMainAction moves keyboard focus to the primary action button.
func (t *VerifyTab) FocusMainAction() {
	t.btnStart.GrabFocus()
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
	t.labelTotalV = widgets.GetLabel(t.Builder, "label_val_total_value")
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

	if err := verify.ValidateChecksumFile(checksumFile); err != nil {
		widgets.ShowError(t.Window, "Invalid Checksum Path",
			fmt.Sprintf("Checksum file is unavailable:\n%s", err))

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

	// Widget reads must complete before the goroutine launches: GTK access off the main thread is UB.
	algoID := t.cmbTxtAlgorithm.GetActiveID()

	lastStats := result.NewVerifierStats()
	currentIdx := int64(0)

	t.activateStopState()

	// Wg.Add(1) immediately after the synchronous validation so a shutdown during the async
	// setup below still waits for the goroutine and never leaks a started RunStream.
	t.Wg.Add(1)

	crash.Go("verifyTab.onStart", func() {
		defer t.Wg.Done()
		defer t.CancelOperation()

		ctx, cancel := context.WithCancel(t.Ctx)
		t.SetCancel(cancel)

		algo, err := algorithm.AlgorithmFromExtension(algoID)
		if err != nil {
			widgets.IdleAdd(t.Window, func() {
				t.setStartState()
				widgets.ShowError(t.Window, "Verification Error",
					fmt.Sprintf("Failed to resolve algorithm: %v", err))
			})

			return
		}

		cfg := verify.VerifyStreamingConfig{
			ChecksumFile: checksumFile,
			Algorithm:    algo,
		}

		results, err := verify.VerifyChecksumsStreaming(ctx, cfg)
		if err != nil {
			widgets.IdleAdd(t.Window, func() {
				t.setStartState()
				widgets.ShowError(t.Window, "Verification Error", fmt.Sprintf("Failed to start verification: %v", err))
			})

			return
		}

		log.Info().
			Str("checksum_file", checksumFile).
			Str("algorithm", cfg.Algorithm.String()).
			Msg("Starting verification")

		appendRows := func(items []verify.VerifyStreamingResult) {
			// Sync idle: the producer waits for the insert, so the visible table and the
			// counters it reports cannot lag the hashing that has already finished.
			widgets.IdleAddSync(ctx, t.Window, func() {
				// Atomic insert emits a single row-inserted per row instead of Append plus one
				// row-changed per column; vals is reused because the values are copied on insert.
				cols := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
				vals := make([]interface{}, len(cols))

				for i := range items {
					r := items[i]
					currentIdx += 1

					note := ""
					if r.Result.Err != nil {
						note = errs.UnwrapAndNormalize(r.Result.Err)
					}

					vals[0] = currentIdx
					vals[1] = r.Result.Path
					vals[2] = bytesize.New(float64(r.Result.ReadBytes)).String()
					vals[3] = r.Result.Status.String()
					vals[4] = r.Result.ActualHash
					vals[5] = r.Result.ExpectedHash
					vals[6] = note
					vals[7] = r.Result.Status.Color()
					vals[8] = r.Result.ReadBytes
					vals[9] = r.Result.FullPath
					vals[10] = r.Result.Status.Priority()

					if err := t.listStore.InsertWithValues(nil, -1, cols, vals); err != nil {
						widgets.MustWidget("ListStore", "VerifyTab.appendRows", err)
					}

					lastStats = r.Stats
				}

				t.updateCounts(lastStats)
			})
		}

		widgets.RunStream(results, widgets.StreamBatchConfig[verify.VerifyStreamingResult]{
			FlushSize:     100,
			FlushInterval: 100 * time.Millisecond,
			IsProgress:    func(r verify.VerifyStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r verify.VerifyStreamingResult) error { return r.Err },
			OnProgress: func(r verify.VerifyStreamingResult) {
				stats := r.Stats

				widgets.IdleAddPriority(t.Window, glib.PRIORITY_HIGH_IDLE, func() {
					t.updateLiveStatus(stats)
				})
			},
			OnBatch: appendRows,
			OnFinish: func(hasError error) {
				widgets.IdleAdd(t.Window, func() {
					// Counters are driven by the inserted batches, so lastStats is the final
					// on-screen snapshot when the stream ends.
					stats := lastStats

					if hasError != nil {
						if errs.IsContextDone(hasError) {
							log.Warn().Msg("Verification canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to verify checksums")
							widgets.ShowError(t.Window, "Verification Error",
								fmt.Sprintf("Failed to verify checksums: %v", hasError))
						}
					} else {
						log.Info().
							Int("matched", stats.Matched).
							Int("mismatch", stats.Mismatch).
							Int("unreadable", stats.Unreadable).
							Int("pending", stats.Pending()).
							Int("total_files", stats.TotalFiles).
							Msg("Verification stats")
						log.Info().Msg("Verification completed")
					}

					t.setStartState()

					if hasError == nil && stats.Mismatch+stats.Unreadable == 0 {
						base.SetFinalLabel(t.labelMatchV, stats.Matched, result.HashMatched.Color())
					}
				})
			},
		})
	})
}

func (t *VerifyTab) onStop() {
	t.CancelOperation()
}

func (t *VerifyTab) activateStopState() {
	lastStats := result.NewVerifierStats()

	t.listStore.Clear()
	t.updateCounts(lastStats)
	t.updateLiveStatus(lastStats)
	t.SuspendSort(t.treeValidate)
	widgets.ResetColumnWidths(t.treeValidate)

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

	t.RestoreSort()
	widgets.ScrollListToTop(t.treeValidate)
	t.onEntryChecksumChanged(false, nil)
}

// updateCounts renders the counters and aggregate bar from stats taken after a table insert,
// so they never run ahead of the visible rows. updateLiveStatus renders the fields driven by
// producer ticks (speed, current file, per-file bar), which move between inserts.
func (t *VerifyTab) updateCounts(stats result.VerifierStats) {
	t.labelMatchV.SetText(fmt.Sprintf("%d files", stats.Matched))
	base.SetStatLabel(t.labelMismatchV, stats.Mismatch, result.HashMismatch.Color())
	base.SetStatLabel(t.labelUnreadableV, stats.Unreadable, result.Unreadable.Color())
	t.labelPendingV.SetText(fmt.Sprintf("%d files", stats.Pending()))
	t.labelTotalV.SetText(fmt.Sprintf("%d files", stats.TotalFiles))
	t.progressTracker.UpdateTotalProgress(stats.TotalProgress())
}

func (t *VerifyTab) updateLiveStatus(stats result.VerifierStats) {
	t.labelSpeedV.SetText(bytesize.New(stats.Speed).String() + "/s")
	t.progressTracker.UpdateCurrentFile(stats.CurrentFileOrStatus)
	t.progressTracker.UpdateFileProgress(stats.FileHashingProgress)
}

func (t *VerifyTab) applySettingsToUI() {
	t.chkBoxVerifyOnOpen.SetActive(t.Settings.Verify.VerifyOnOpen)
	t.ColumnConfig.ApplyColumnOrder(t.treeValidate, t.Settings.Verify.ColumnOrder)
	t.ApplySortOrder(t.treeValidate, t.Settings.Verify.SortColumn, t.Settings.Verify.SortOrder)
}

func (t *VerifyTab) saveSettings() error {
	if !widgets.IsAlive(t.Window) {
		return nil
	}

	t.Settings.Verify.VerifyOnOpen = t.chkBoxVerifyOnOpen.GetActive()
	t.Settings.Verify.ColumnOrder = t.ColumnConfig.GetColumnOrder(t.treeValidate)

	// While a bulk run has sorting suspended the indicators are hidden, so GetSortState would
	// report "" and overwrite the persisted column; keep the saved value in that window.
	if !t.SortSuspended() {
		sortColumn, sortOrder := t.ColumnConfig.GetSortState(t.treeValidate)

		t.Settings.Verify.SortColumn = sortColumn
		if sortOrder == gtk.SORT_DESCENDING {
			t.Settings.Verify.SortOrder = settings.SortOrderDesc
		} else {
			t.Settings.Verify.SortOrder = settings.SortOrderAsc
		}
	}

	return t.Settings.AsyncSave()
}

func (t *VerifyTab) setupContextMenu() {
	columnLabels := []string{"index", "path", "size", "status", "hash", "expected hash", "note"}
	t.contextMenuProvider.CreateMenuWithReveal(9, columnLabels, t.revealSelectedFile)
	t.contextMenuProvider.ConnectRightClick(func() {
		t.contextMenuProvider.ShowMenu()
	})
}

func (t *VerifyTab) revealSelectedFile(fullPath string) {
	crash.Go("verifyTab.revealSelectedFile", func() {
		if err := reveal.Reveal(t.Ctx, fullPath); err != nil {
			widgets.IdleAdd(t.Window, func() {
				widgets.ShowError(t.Window, "Reveal Error",
					fmt.Sprintf("Failed to open file manager:\n%v", err))
			})

			return
		}
	})
}

func (t *VerifyTab) onEntryChecksumChanged(updateActiveID bool, onStartFunc func()) {
	path, _ := t.entryChecksum.GetText()

	_, extErr := algorithm.AlgorithmFromExtension(path)
	foundByExt := extErr == nil

	// Resolve through the shared chain (same as the app gate and CLI) so a routed
	// file never lands on ".unknown"; foundByExt only drives combo sensitivity.
	algo, err := algorithm.ResolveAlgorithmFromFile(path)
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

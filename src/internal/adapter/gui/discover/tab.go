// Package discover implements the Discover GUI tab: it opens the sources dialog, runs a streaming
// scan that compares each file's CRC-32 with the checksum embedded in its name, and renames files
// from the discovered results.
package discover

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/reveal"
	"github.com/ostapkonst/HashVerifier/internal/service/discover"
)

// DiscoverTab embeds base.TabBase and holds the sources, scan filter, and last discovery results.
type DiscoverTab struct {
	*base.TabBase
	btnDiscover          *gtk.Button
	btnRename            *gtk.Button
	btnStop              *gtk.Button
	treeDiscover         *gtk.TreeView
	listStore            *gtk.ListStore
	contextMenuProvider  *widgets.ContextMenuProvider
	progressTracker      *base.ProgressTracker
	frameSummary         *gtk.Frame
	frameRenameSummary   *gtk.Frame
	gridRenameProgress   *gtk.Grid
	renameProgressBar    *gtk.ProgressBar
	labelMatchV          *gtk.Label
	labelNoChecksumV     *gtk.Label
	labelMismatchV       *gtk.Label
	labelFailedV         *gtk.Label
	labelPendingV        *gtk.Label
	labelSpeedV          *gtk.Label
	labelRenameRenamedV  *gtk.Label
	labelRenameFailedV   *gtk.Label
	labelRenamePendingV  *gtk.Label
	labelRenameSpeedV    *gtk.Label
	labelRenameCurrFileV *gtk.Label

	sources    []widgets.DiscoverSource
	filter     embedded.FilterMode
	renameMode embedded.RenameMode
	results    []embedded.RenameCandidate
	stats      result.DiscovererStats
}

// NewDiscoverTab initializes the Discover tab's widgets, restores persisted settings, and connects every handler.
func NewDiscoverTab(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings) *DiscoverTab {
	tab := &DiscoverTab{
		TabBase: base.NewTabBase(ctx, builder, window, settings, widgets.NewDiscoverColumnConfig()),
	}
	tab.getWidgets()
	tab.getLabels()
	tab.progressTracker = base.NewProgressTracker(
		tab.Builder,
		"grid_discover_progress",
		"progress_discover_total",
		"progress_discover_curr_file",
		"label_discover_curr_file_value",
	)
	tab.contextMenuProvider = widgets.NewContextMenuProvider(tab.treeDiscover, tab.listStore)
	tab.applySettingsToUI()
	tab.setStartState()
	tab.setupHandlers()

	return tab
}

// Sources returns the paths of every selected source, in list order.
func (t *DiscoverTab) Sources() []string {
	paths := make([]string, 0, len(t.sources))

	for _, src := range t.sources {
		paths = append(paths, src.Path)
	}

	return paths
}

func (t *DiscoverTab) getWidgets() {
	t.btnDiscover = widgets.GetButton(t.Builder, "btn_discover_sources")
	t.btnRename = widgets.GetButton(t.Builder, "btn_rename_files")
	t.btnStop = widgets.GetButton(t.Builder, "btn_stop_discover")
	t.treeDiscover = widgets.GetTreeView(t.Builder, "tree_discover")
	t.listStore = widgets.GetListStore(t.Builder, "liststore_discover")
	t.frameSummary = widgets.GetFrame(t.Builder, "frame_discover_summary")
	t.frameRenameSummary = widgets.GetFrame(t.Builder, "frame_discover_rename_summary")
	t.gridRenameProgress = widgets.GetGrid(t.Builder, "grid_discover_rename_progress")
	t.renameProgressBar = widgets.GetProgressBar(t.Builder, "progress_discover_rename_total")
}

func (t *DiscoverTab) getLabels() {
	t.labelMatchV = widgets.GetLabel(t.Builder, "label_discover_match_value")
	t.labelNoChecksumV = widgets.GetLabel(t.Builder, "label_discover_no_checksum_value")
	t.labelMismatchV = widgets.GetLabel(t.Builder, "label_discover_mismatch_value")
	t.labelFailedV = widgets.GetLabel(t.Builder, "label_discover_failed_value")
	t.labelPendingV = widgets.GetLabel(t.Builder, "label_discover_pending_value")
	t.labelSpeedV = widgets.GetLabel(t.Builder, "label_discover_speed_value")
	t.labelRenameRenamedV = widgets.GetLabel(t.Builder, "label_discover_rename_renamed_value")
	t.labelRenameFailedV = widgets.GetLabel(t.Builder, "label_discover_rename_failed_value")
	t.labelRenamePendingV = widgets.GetLabel(t.Builder, "label_discover_rename_pending_value")
	t.labelRenameSpeedV = widgets.GetLabel(t.Builder, "label_discover_rename_speed_value")
	t.labelRenameCurrFileV = widgets.GetLabel(t.Builder, "label_discover_rename_curr_file_value")
}

func (t *DiscoverTab) setupHandlers() {
	t.btnDiscover.Connect("clicked", t.onDiscoverClicked)
	t.btnRename.Connect("clicked", t.onRenameClicked)
	t.btnStop.Connect("clicked", func() { t.CancelOperation() })
	t.setupContextMenu()
	t.SetupColumnHandlers(t.treeDiscover, func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save discover settings", err)
		}
	})
}

func (t *DiscoverTab) onDiscoverClicked() {
	dlg := widgets.NewDiscoverSourcesDialog(
		t.Window,
		"Discover Files",
		widgets.DiscoverOptions{
			Sources: t.sources,
			Filter:  t.filter,
		},
		t.Settings.Discover.SourcesDialog.Width,
		t.Settings.Discover.SourcesDialog.Height,
	)
	defer dlg.Destroy()

	options, ok := dlg.Run()

	if ok {
		t.sources = options.Sources
		t.filter = options.Filter

		t.Settings.Discover.SourcesDialog.Filter = t.filter.String()
	}

	t.saveSourcesDialogSize(dlg)

	if !ok {
		return
	}

	t.onStart()
}

func (t *DiscoverTab) onStart() {
	sources := t.Sources()

	if err := discover.ValidateSources(sources); err != nil {
		widgets.ShowError(t.Window, "Invalid Sources",
			fmt.Sprintf("Cannot scan the selected sources:\n%s", err))

		return
	}

	filter := t.filter

	lastStats := result.NewDiscovererStats()
	currentIdx := int64(0)

	t.activateStopState()

	t.Wg.Add(1)

	crash.Go("discoverTab.onStart", func() {
		defer t.Wg.Done()
		defer t.CancelOperation()

		ctx, cancel := context.WithCancel(t.Ctx)
		t.SetCancel(cancel)

		cfg := discover.DiscoverStreamingConfig{
			Sources: sources,
			Filter:  filter,
		}

		results, err := discover.DiscoverStreaming(ctx, cfg)
		if err != nil {
			widgets.IdleAdd(t.Window, func() {
				t.setStartState()
				widgets.ShowError(t.Window, "Discovery Error", fmt.Sprintf("Failed to start discovery: %v", err))
			})

			return
		}

		log.Info().
			Strs("sources", cfg.Sources).
			Str("filter", cfg.Filter.String()).
			Msg("Starting discovery")

		var collected []embedded.RenameCandidate

		appendRows := func(items []discover.DiscoverStreamingResult) {
			widgets.IdleAddSync(ctx, t.Window, func() {
				cols := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
				vals := make([]interface{}, len(cols))

				for i := range items {
					r := items[i]
					currentIdx++

					note := ""
					if r.Result.Err != nil {
						note = errs.UnwrapAndNormalize(r.Result.Err)
					}

					vals[0] = currentIdx
					vals[1] = r.Result.Path
					vals[2] = bytesize.New(float64(r.Result.ReadBytes)).String()
					vals[3] = r.Result.Status.String()
					vals[4] = r.Result.ActualHash
					vals[5] = r.Result.EmbeddedHash
					vals[6] = note
					vals[7] = r.Result.Status.Color()
					vals[8] = r.Result.ReadBytes
					vals[9] = r.Result.Status.Priority()

					if err := t.listStore.InsertWithValues(nil, -1, cols, vals); err != nil {
						widgets.MustWidget("ListStore", "DiscoverTab.appendRows", err)
					}

					// Failed files have no computed checksum, so they cannot be renamed.
					if r.Result.Status != result.DiscoverFailed {
						collected = append(collected, embedded.RenameCandidate{
							Path:     r.Result.Path,
							Checksum: r.Result.ActualHash,
						})
					}

					lastStats = r.Stats
				}

				t.updateCounts(lastStats)
			})
		}

		widgets.RunStream(results, widgets.StreamBatchConfig[discover.DiscoverStreamingResult]{
			FlushSize:     100,
			FlushInterval: 100 * time.Millisecond,
			IsProgress:    func(r discover.DiscoverStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r discover.DiscoverStreamingResult) error { return r.Err },
			OnProgress: func(r discover.DiscoverStreamingResult) {
				stats := r.Stats

				widgets.IdleAddPriority(t.Window, glib.PRIORITY_HIGH_IDLE, func() {
					t.updateLiveStatus(stats)
				})
			},
			OnBatch: appendRows,
			OnFinish: func(hasError error) {
				widgets.IdleAdd(t.Window, func() {
					stats := lastStats

					if hasError != nil {
						if errs.IsContextDone(hasError) {
							log.Warn().Msg("Discovery canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to discover files")
							widgets.ShowError(t.Window, "Discovery Error",
								fmt.Sprintf("Failed to discover files: %v", hasError))
						}
					} else {
						log.Info().
							Int("matched", stats.Matched).
							Int("mismatch", stats.Mismatch).
							Int("failed", stats.Failed).
							Int("no_checksum", stats.NoChecksum).
							Int("pending", stats.Pending()).
							Int("total_files", stats.TotalFiles).
							Msg("Discovery stats")
						log.Info().Msg("Discovery completed")
					}

					t.stats = stats

					t.results = collected
					t.setStartState()
				})
			},
		})
	})
}

func (t *DiscoverTab) onRenameClicked() {
	dlg := widgets.NewRenameFilesDialog(
		t.Window,
		"Rename Files",
		t.results,
		t.renameMode,
		t.Settings.Discover.RenameDialog.Width,
		t.Settings.Discover.RenameDialog.Height,
	)
	defer dlg.Destroy()

	options, ok := dlg.Run()

	if ok {
		t.renameMode = options.Mode

		t.Settings.Discover.RenameDialog.Mode = t.renameMode.String()
	}

	t.saveRenameDialogSize(dlg)

	if !ok {
		return
	}

	t.applyRenames(options.Actions)
}

// applyRenames renames the files on disk off the GTK thread and, on success, updates the affected
// table rows and the stored results so a later rename dialog sees the new names.
func (t *DiscoverTab) applyRenames(actions []embedded.RenameAction) {
	mode := t.renameMode

	log.Info().
		Int("files", len(actions)).
		Str("mode", mode.String()).
		Msg("Starting rename")

	rowIters, resultIdx := t.buildRenameIndexes()

	t.activateRenameSummary(len(actions))
	t.setBusyState()
	t.SuspendSort(t.treeDiscover)

	t.Wg.Add(1)

	crash.Go("discoverTab.rename", func() {
		defer t.Wg.Done()
		defer t.CancelOperation()

		ctx, cancel := context.WithCancel(t.Ctx)
		t.SetCancel(cancel)

		renamed := 0
		failed := 0

		var failureLines []string

		widgets.RunStream(discover.RenameStreaming(ctx, actions), widgets.StreamBatchConfig[discover.RenameStreamingResult]{
			FlushSize:     100,
			FlushInterval: 100 * time.Millisecond,
			IsProgress:    func(r discover.RenameStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r discover.RenameStreamingResult) error { return r.Err },
			OnProgress: func(r discover.RenameStreamingResult) {
				stats := r.Stats

				widgets.IdleAddPriority(t.Window, glib.PRIORITY_HIGH_IDLE, func() {
					t.updateRenameProgress(stats)
				})
			},
			OnBatch: func(items []discover.RenameStreamingResult) {
				widgets.IdleAddSync(ctx, t.Window, func() {
					for _, r := range items {
						if r.Outcome.Err != nil {
							failed++

							failureLines = append(failureLines,
								fmt.Sprintf("%s: %v", r.Outcome.Action.From, r.Outcome.Err))

							continue
						}

						renamed++

						t.updateRenamedRow(r.Outcome.Action, rowIters, resultIdx)
					}

					if n := len(items); n > 0 {
						t.updateRenameSummary(items[n-1].Stats)
						t.updateRenameProgress(items[n-1].Stats)
					}
				})
			},
			OnFinish: func(hasError error) {
				widgets.IdleAdd(t.Window, func() {
					if hasError != nil {
						if errs.IsContextDone(hasError) {
							log.Warn().Msg("Rename canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to rename files")
						}
					} else {
						log.Info().
							Int("renamed", renamed).
							Int("failed", failed).
							Int("pending", len(actions)-renamed-failed).
							Int("total", len(actions)).
							Msg("Rename stats")
						log.Info().Msg("Rename completed")
					}

					t.restoreDiscoverySummary()

					t.stats = t.recountStats()
					t.updateCounts(t.stats)

					t.setStartState()

					if hasError == nil && failed > 0 {
						widgets.ShowError(t.Window, "Rename Error",
							fmt.Sprintf("%d file(s) could not be renamed:\n%s",
								failed, strings.Join(failureLines, "\n")))
					}
				})
			},
		})
	})
}

// updateRenamedRow rewrites the row whose full path is action.From to reflect the new name; the
// content hash is unchanged, so only the path and status-related columns move.
func (t *DiscoverTab) updateRenamedRow(action embedded.RenameAction, rowIters map[string]*gtk.TreeIter, resultIdx map[string]int) {
	iter, ok := rowIters[action.From]
	if !ok {
		return
	}

	actual, _ := widgets.ListStoreString(t.listStore, iter, 4)
	_, checksum, _, hasChecksum := embedded.Split(filepath.Base(action.To))

	status := result.DiscoverNoChecksum
	embeddedHash := ""

	if hasChecksum {
		embeddedHash = strings.ToLower(checksum)

		if strings.EqualFold(actual, checksum) {
			status = result.DiscoverMatched
		} else {
			status = result.DiscoverMismatch
		}
	}

	values := map[int]interface{}{
		1: action.To,
		3: status.String(),
		5: embeddedHash,
		6: "",
		7: status.Color(),
		9: status.Priority(),
	}

	for col, value := range values {
		if err := t.listStore.SetValue(iter, col, value); err != nil {
			widgets.MustWidget("ListStore", "DiscoverTab.updateRenamedRow", err)
		}
	}

	if idx, ok := resultIdx[action.From]; ok {
		t.results[idx].Path = action.To
		delete(resultIdx, action.From)
		resultIdx[action.To] = idx
	}

	delete(rowIters, action.From)
	rowIters[action.To] = iter
}

// buildRenameIndexes maps row paths to their iterators and result indexes so a rename updates
// rows in O(1) instead of rescanning the table per file. Must be called on the GTK thread.
func (t *DiscoverTab) buildRenameIndexes() (map[string]*gtk.TreeIter, map[string]int) {
	rowIters := make(map[string]*gtk.TreeIter, len(t.results))

	iter, ok := t.listStore.GetIterFirst()
	for ok {
		if path, valid := widgets.ListStoreString(t.listStore, iter, 1); valid {
			it := *iter
			rowIters[path] = &it
		}

		ok = t.listStore.IterNext(iter)
	}

	resultIdx := make(map[string]int, len(t.results))
	for i := range t.results {
		resultIdx[t.results[i].Path] = i
	}

	return rowIters, resultIdx
}

func (t *DiscoverTab) updateRenameSummary(stats result.RenameStats) {
	t.labelRenameRenamedV.SetText(fmt.Sprintf("%d of %d files", stats.Renamed, stats.TotalFiles))
	base.SetStatLabel(t.labelRenameFailedV, stats.Failed, stats.TotalFiles, result.DiscoverFailed.Color())
	t.labelRenamePendingV.SetText(fmt.Sprintf("%d of %d files", stats.Pending(), stats.TotalFiles))
	t.labelRenameSpeedV.SetText(fmt.Sprintf("%.1f files/s", stats.Speed))
}

func (t *DiscoverTab) updateRenameProgress(stats result.RenameStats) {
	t.renameProgressBar.SetFraction(stats.TotalProgress())
	t.labelRenameCurrFileV.SetText(stats.CurrentFileOrStatus)
}

func (t *DiscoverTab) activateStopState() {
	stats := result.NewDiscovererStats()

	t.listStore.Clear()
	t.results = nil
	t.stats = stats
	t.updateCounts(stats)
	t.updateLiveStatus(stats)
	t.SuspendSort(t.treeDiscover)
	widgets.ResetColumnWidths(t.treeDiscover)

	t.setBusyState()
	t.progressTracker.ActivateStopState()
}

// setBusyState hides the start actions and shows Stop without touching progress widgets, so each
// flow can pick its own progress area.
func (t *DiscoverTab) setBusyState() {
	t.btnDiscover.SetVisible(false)
	t.btnRename.SetVisible(false)
	t.btnStop.SetVisible(true)
}

// activateRenameSummary swaps the discovery Summary for the rename counters and reveals the
// rename progress area: overall progress plus the current file/status caption.
func (t *DiscoverTab) activateRenameSummary(total int) {
	t.frameSummary.SetVisible(false)
	t.frameRenameSummary.SetVisible(true)

	t.labelRenameRenamedV.SetText(fmt.Sprintf("0 of %d files", total))
	t.labelRenameFailedV.SetText(fmt.Sprintf("0 of %d files", total))
	t.labelRenamePendingV.SetText(fmt.Sprintf("%d of %d files", total, total))
	t.labelRenameSpeedV.SetText("0.0 files/s")
	t.labelRenameCurrFileV.SetText("renaming files...")

	t.progressTracker.SetStartState()
	t.gridRenameProgress.SetVisible(true)
	t.renameProgressBar.SetFraction(0)
}

// restoreDiscoverySummary swaps the discovery Summary back and hides the rename progress area.
func (t *DiscoverTab) restoreDiscoverySummary() {
	t.frameRenameSummary.SetVisible(false)
	t.frameSummary.SetVisible(true)
	t.gridRenameProgress.SetVisible(false)
}

// setStartState restores the idle state: Discover available, Stop hidden, and Rename enabled only
// once a discovery run has produced results to rename.
func (t *DiscoverTab) setStartState() {
	t.btnDiscover.SetVisible(true)
	t.btnRename.SetVisible(true)
	t.btnStop.SetVisible(false)
	t.btnRename.SetSensitive(len(t.results) > 0)
	t.progressTracker.SetStartState()

	t.RestoreSort()
	widgets.ScrollListToTop(t.treeDiscover)
}

// updateCounts renders the counters and aggregate bar from stats taken after a table insert,
// so they never run ahead of the visible rows. updateLiveStatus renders the fields driven by
// producer ticks (speed, current file, per-file bar), which move between inserts.
func (t *DiscoverTab) updateCounts(stats result.DiscovererStats) {
	t.labelMatchV.SetText(fmt.Sprintf("%d of %d files", stats.Matched, stats.TotalFiles))
	t.labelNoChecksumV.SetText(fmt.Sprintf("%d of %d files", stats.NoChecksum, stats.TotalFiles))
	base.SetStatLabel(t.labelMismatchV, stats.Mismatch, stats.TotalFiles, result.DiscoverMismatch.Color())
	base.SetStatLabel(t.labelFailedV, stats.Failed, stats.TotalFiles, result.DiscoverFailed.Color())
	t.labelPendingV.SetText(fmt.Sprintf("%d of %d files", stats.Pending(), stats.TotalFiles))
	t.progressTracker.UpdateTotalProgress(stats.TotalProgress())
}

func (t *DiscoverTab) updateLiveStatus(stats result.DiscovererStats) {
	t.labelSpeedV.SetText(bytesize.New(stats.Speed).String() + "/s")
	t.progressTracker.UpdateCurrentFile(stats.CurrentFileOrStatus)
	t.progressTracker.UpdateFileProgress(stats.FileHashingProgress)
}

// recountStats rebuilds the status counters from the current table rows, keeping TotalFiles, so
// the Summary stays accurate after renames change rows in place.
func (t *DiscoverTab) recountStats() result.DiscovererStats {
	stats := t.stats
	stats.Matched = 0
	stats.Mismatch = 0
	stats.Failed = 0
	stats.NoChecksum = 0

	iter, ok := t.listStore.GetIterFirst()
	for ok {
		status, _ := widgets.ListStoreString(t.listStore, iter, 3)

		switch status {
		case result.DiscoverMatched.String():
			stats.Matched++
		case result.DiscoverMismatch.String():
			stats.Mismatch++
		case result.DiscoverFailed.String():
			stats.Failed++
		case result.DiscoverNoChecksum.String():
			stats.NoChecksum++
		}

		ok = t.listStore.IterNext(iter)
	}

	return stats
}

func (t *DiscoverTab) setupContextMenu() {
	columnLabels := []string{"index", "path", "size", "status", "hash", "expected hash", "note"}
	t.contextMenuProvider.CreateMenuWithReveal(1, columnLabels, t.revealSelectedFile)
	t.contextMenuProvider.ConnectRightClick(func() {
		t.contextMenuProvider.ShowMenu()
	})
}

func (t *DiscoverTab) revealSelectedFile(fullPath string) {
	crash.Go("discoverTab.revealSelectedFile", func() {
		if err := reveal.Reveal(t.Ctx, fullPath); err != nil {
			widgets.IdleAdd(t.Window, func() {
				widgets.ShowError(t.Window, "Reveal Error",
					fmt.Sprintf("Failed to open file manager:\n%v", err))
			})

			return
		}
	})
}

func (t *DiscoverTab) applySettingsToUI() {
	t.ColumnConfig.ApplyColumnOrder(t.treeDiscover, t.Settings.Discover.ColumnOrder)
	t.ApplySortOrder(t.treeDiscover, t.Settings.Discover.SortColumn, t.Settings.Discover.SortOrder)

	if mode, ok := embedded.FilterModeFromString(t.Settings.Discover.SourcesDialog.Filter); ok {
		t.filter = mode
	}

	if mode, ok := embedded.RenameModeFromString(t.Settings.Discover.RenameDialog.Mode); ok {
		t.renameMode = mode
	}
}

func (t *DiscoverTab) saveSettings() error {
	if !widgets.IsAlive(t.Window) {
		return nil
	}

	t.Settings.Discover.ColumnOrder = t.ColumnConfig.GetColumnOrder(t.treeDiscover)

	// While a bulk run has sorting suspended the indicators are hidden, so GetSortState would
	// report "" and overwrite the persisted column; keep the saved value in that window.
	if !t.SortSuspended() {
		sortColumn, sortOrder := t.ColumnConfig.GetSortState(t.treeDiscover)

		t.Settings.Discover.SortColumn = sortColumn
		if sortOrder == gtk.SORT_DESCENDING {
			t.Settings.Discover.SortOrder = settings.SortOrderDesc
		} else {
			t.Settings.Discover.SortOrder = settings.SortOrderAsc
		}
	}

	return t.Settings.AsyncSave()
}

func (t *DiscoverTab) saveSourcesDialogSize(dlg *widgets.DiscoverSourcesDialog) {
	w, h := dlg.GetSize()
	t.Settings.Discover.SourcesDialog.Width = w
	t.Settings.Discover.SourcesDialog.Height = h

	if err := t.Settings.AsyncSave(); err != nil {
		t.LogError("save discover sources dialog size", err)
	}
}

func (t *DiscoverTab) saveRenameDialogSize(dlg *widgets.RenameFilesDialog) {
	w, h := dlg.GetSize()
	t.Settings.Discover.RenameDialog.Width = w
	t.Settings.Discover.RenameDialog.Height = h

	if err := t.Settings.AsyncSave(); err != nil {
		t.LogError("save discover rename dialog size", err)
	}
}

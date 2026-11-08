// Package discover implements the Discover GUI tab: it opens the sources dialog and runs a
// streaming scan that compares each file's CRC-32 with the checksum embedded in its name.
package discover

import (
	"context"
	"fmt"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/reveal"
	"github.com/ostapkonst/HashVerifier/internal/service/discover"
)

// DiscoverTab embeds base.TabBase and holds the sources and scan options.
type DiscoverTab struct {
	*base.TabBase
	btnStart            *gtk.Button
	btnStop             *gtk.Button
	treeDiscover        *gtk.TreeView
	listStore           *gtk.ListStore
	contextMenuProvider *widgets.ContextMenuProvider
	progressTracker     *base.ProgressTracker
	labelMatchV         *gtk.Label
	labelMismatchV      *gtk.Label
	labelFailedV        *gtk.Label
	labelSkippedV       *gtk.Label
	labelPendingV       *gtk.Label
	labelTotalV         *gtk.Label
	labelSpeedV         *gtk.Label

	sources        []widgets.DiscoverSource
	followSymlinks bool
	skipHidden     bool
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

// sourcePaths returns the paths of every selected source, in list order.
func (t *DiscoverTab) sourcePaths() []string {
	paths := make([]string, 0, len(t.sources))

	for _, src := range t.sources {
		paths = append(paths, src.Path)
	}

	return paths
}

func (t *DiscoverTab) getWidgets() {
	t.btnStart = widgets.GetButton(t.Builder, "btn_discover_sources")
	t.btnStop = widgets.GetButton(t.Builder, "btn_stop_discover")
	t.treeDiscover = widgets.GetTreeView(t.Builder, "tree_discover")
	t.listStore = widgets.GetListStore(t.Builder, "liststore_discover")
}

func (t *DiscoverTab) getLabels() {
	t.labelMatchV = widgets.GetLabel(t.Builder, "label_discover_match_value")
	t.labelMismatchV = widgets.GetLabel(t.Builder, "label_discover_mismatch_value")
	t.labelFailedV = widgets.GetLabel(t.Builder, "label_discover_failed_value")
	t.labelSkippedV = widgets.GetLabel(t.Builder, "label_discover_skipped_value")
	t.labelPendingV = widgets.GetLabel(t.Builder, "label_discover_pending_value")
	t.labelTotalV = widgets.GetLabel(t.Builder, "label_discover_total_value")
	t.labelSpeedV = widgets.GetLabel(t.Builder, "label_discover_speed_value")
}

func (t *DiscoverTab) setupHandlers() {
	t.btnStart.Connect("clicked", t.onStart)
	t.btnStop.Connect("clicked", t.onStop)
	t.setupContextMenu()
	t.SetupColumnHandlers(t.treeDiscover, func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save discover settings", err)
		}
	})
}

func (t *DiscoverTab) onStart() {
	dlg := widgets.NewDiscoverSourcesDialog(
		t.Window,
		"Discover Files",
		widgets.DiscoverOptions{
			Sources:             t.sources,
			FollowSymbolicLinks: t.followSymlinks,
			SkipHidden:          t.skipHidden,
		},
		t.Settings.Discover.SourcesDialog.Width,
		t.Settings.Discover.SourcesDialog.Height,
	)
	defer dlg.Destroy()

	options, ok := dlg.Run()

	if ok {
		t.sources = options.Sources
		t.followSymlinks = options.FollowSymbolicLinks
		t.skipHidden = options.SkipHidden

		t.Settings.Discover.SourcesDialog.FollowSymbolicLinks = t.followSymlinks
		t.Settings.Discover.SourcesDialog.SkipHidden = t.skipHidden
	}

	t.saveSourcesDialogSize(dlg)

	if !ok {
		return
	}

	t.runDiscovery()
}

func (t *DiscoverTab) runDiscovery() {
	sources := t.sourcePaths()

	if !t.validateInputs(sources) {
		return
	}

	followSymlinks := t.followSymlinks
	skipHidden := t.skipHidden

	lastStats := result.NewDiscovererStats()
	currentIdx := int64(0)

	t.activateStopState()

	// Wg.Add(1) immediately after the synchronous validation so a shutdown during the async
	// setup below still waits for the goroutine and never leaks a started RunStream.
	t.Wg.Add(1)

	crash.Go("discoverTab.runDiscovery", func() {
		defer t.Wg.Done()
		defer t.CancelOperation()

		ctx, cancel := context.WithCancel(t.Ctx)
		t.SetCancel(cancel)

		cfg := discover.DiscoverStreamingConfig{
			Sources:             sources,
			FollowSymbolicLinks: followSymlinks,
			SkipHidden:          skipHidden,
		}

		results, err := discover.DiscoverFilesStreaming(ctx, cfg)
		if err != nil {
			widgets.IdleAdd(t.Window, func() {
				t.setStartState()
				widgets.ShowError(t.Window, "Discovery Error", fmt.Sprintf("Failed to start discovery: %v", err))
			})

			return
		}

		log.Info().
			Strs("sources", cfg.Sources).
			Bool("follow_symbolic_links", cfg.FollowSymbolicLinks).
			Bool("skip_hidden", cfg.SkipHidden).
			Msg("Starting discovery")

		appendRows := func(items []discover.DiscoverStreamingResult) {
			// Sync idle: the producer waits for the insert, so the visible table and the
			// counters it reports cannot lag the hashing that has already finished.
			widgets.IdleAddSync(ctx, t.Window, func() {
				// Atomic insert emits a single row-inserted per row instead of Append plus one
				// row-changed per column; vals is reused because the values are copied on insert.
				cols := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
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
					vals[5] = r.Result.EmbeddedHash
					vals[6] = note
					vals[7] = r.Result.Status.Color()
					vals[8] = r.Result.ReadBytes
					vals[9] = r.Result.Status.Priority()

					if err := t.listStore.InsertWithValues(nil, -1, cols, vals); err != nil {
						widgets.MustWidget("ListStore", "DiscoverTab.appendRows", err)
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
					// Counters are driven by the inserted batches, so lastStats is the final
					// on-screen snapshot when the stream ends.
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
							Int("skipped", stats.Skipped).
							Int("pending", stats.Pending()).
							Int("total_files", stats.TotalFiles).
							Msg("Discovery stats")
						log.Info().Msg("Discovery completed")
					}

					t.setStartState()

					if hasError == nil && stats.Mismatch+stats.Failed == 0 {
						base.SetFinalLabel(t.labelMatchV, stats.Matched, result.DiscoverMatched.Color())
					}
				})
			},
		})
	})
}

func (t *DiscoverTab) onStop() {
	t.CancelOperation()
}

func (t *DiscoverTab) validateInputs(sources []string) bool {
	if err := discover.ValidateSources(sources); err != nil {
		widgets.ShowError(t.Window, "Invalid Source Path",
			fmt.Sprintf("Selected sources are unavailable:\n%s", err))

		return false
	}

	return true
}

func (t *DiscoverTab) activateStopState() {
	stats := result.NewDiscovererStats()

	t.listStore.Clear()
	t.updateCounts(stats)
	t.updateLiveStatus(stats)
	t.SuspendSort(t.treeDiscover)
	widgets.ResetColumnWidths(t.treeDiscover)

	t.btnStart.SetVisible(false)
	t.btnStop.SetVisible(true)
	t.progressTracker.ActivateStopState()
}

// setStartState restores the idle state: Discover available and Stop hidden.
func (t *DiscoverTab) setStartState() {
	t.btnStart.SetVisible(true)
	t.btnStop.SetVisible(false)
	t.progressTracker.SetStartState()

	t.RestoreSort()
	widgets.ScrollListToTop(t.treeDiscover)
}

// updateCounts renders the counters and aggregate bar from stats taken after a table insert,
// so they never run ahead of the visible rows. updateLiveStatus renders the fields driven by
// producer ticks (speed, current file, per-file bar), which move between inserts.
func (t *DiscoverTab) updateCounts(stats result.DiscovererStats) {
	t.labelMatchV.SetText(fmt.Sprintf("%d files", stats.Matched))
	base.SetStatLabel(t.labelMismatchV, stats.Mismatch, result.DiscoverMismatch.Color())
	base.SetStatLabel(t.labelFailedV, stats.Failed, result.DiscoverFailed.Color())
	base.SetStatLabel(t.labelSkippedV, stats.Skipped, result.DiscoverSkipped.Color())
	t.labelPendingV.SetText(fmt.Sprintf("%d files", stats.Pending()))
	t.labelTotalV.SetText(fmt.Sprintf("%d files", stats.TotalFiles))
	t.progressTracker.UpdateTotalProgress(stats.TotalProgress())
}

func (t *DiscoverTab) updateLiveStatus(stats result.DiscovererStats) {
	t.labelSpeedV.SetText(bytesize.New(stats.Speed).String() + "/s")
	t.progressTracker.UpdateCurrentFile(stats.CurrentFileOrStatus)
	t.progressTracker.UpdateFileProgress(stats.FileHashingProgress)
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

	t.followSymlinks = t.Settings.Discover.SourcesDialog.FollowSymbolicLinks
	t.skipHidden = t.Settings.Discover.SourcesDialog.SkipHidden
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

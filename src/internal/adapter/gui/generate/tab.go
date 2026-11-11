// Package generate implements the Generate GUI tab: input dir + output file pickers,
// options, and a streaming table of hashed files.
package generate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
	"github.com/ostapkonst/HashVerifier/internal/platform/reveal"
	"github.com/ostapkonst/HashVerifier/internal/service/generate"
)

// GenerateTab embeds base.TabBase and adds GTK widgets and callbacks for the Generate use-case.
type GenerateTab struct {
	*base.TabBase
	entryDir             *gtk.Entry
	btnStart             *gtk.Button
	btnStop              *gtk.Button
	btnBrowseDir         *gtk.Button
	treeGenerate         *gtk.TreeView
	listStore            *gtk.ListStore
	entryChecksum        *gtk.Entry
	btnSaveChk           *gtk.Button
	cmbTxtAlgorithm      *gtk.ComboBoxText
	chkBtnFollowSymlinks *gtk.CheckButton
	chkBtnSortPaths      *gtk.CheckButton
	chkBtnFlatPaths      *gtk.CheckButton
	chkBtnSkipHidden     *gtk.CheckButton
	contextMenuProvider  *widgets.ContextMenuProvider
	progressTracker      *base.ProgressTracker
	labelProcessedV      *gtk.Label
	labelSkippedV        *gtk.Label
	labelWithErrorsV     *gtk.Label
	labelPendingV        *gtk.Label
	labelTotalV          *gtk.Label
	labelSpeedV          *gtk.Label

	btnExclude   *gtk.LinkButton
	excludeRules []exclude.Rule
}

// NewGenerateTab initializes the Generate tab's widgets, restores persisted settings, and connects every handler.
func NewGenerateTab(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings) *GenerateTab {
	tab := &GenerateTab{
		TabBase: base.NewTabBase(ctx, builder, window, settings, widgets.NewGenerateColumnConfig()),
	}
	tab.getWidgets()
	tab.getLabels()
	tab.progressTracker = base.NewProgressTracker(
		tab.Builder,
		"grid_gen_progress",
		"progress_gen_total",
		"progress_gen_curr_file",
		"label_gen_curr_file_value",
	)
	tab.contextMenuProvider = widgets.NewContextMenuProvider(tab.treeGenerate, tab.listStore)
	tab.setupExcludeCSS()
	tab.applySettingsToUI()
	tab.setStartState()
	tab.setupHandlers()

	return tab
}

// Fill populates the input-dir and output-file fields from path, choosing flat or
// hierarchical output per the current UI state.
// Returns base.ErrTabBusy if the tab is currently running.
func (t *GenerateTab) Fill(path string) error {
	if t.IsBusy() {
		return base.ErrTabBusy
	}

	t.entryDir.SetText(path)
	extension := t.cmbTxtAlgorithm.GetActiveID()

	if t.chkBtnFlatPaths.GetActive() {
		t.entryChecksum.SetText(widgets.GenChecksumFilenameFlat(path, extension))
	} else {
		t.entryChecksum.SetText(widgets.GenChecksumFilename(path, extension))
	}

	return nil
}

// FocusMainAction moves keyboard focus to the primary action button.
func (t *GenerateTab) FocusMainAction() {
	t.btnStart.GrabFocus()
}

func (t *GenerateTab) getWidgets() {
	t.entryDir = widgets.GetEntry(t.Builder, "entry_gen_dir")
	t.btnStart = widgets.GetButton(t.Builder, "btn_start_generate")
	t.btnStop = widgets.GetButton(t.Builder, "btn_stop_generate")
	t.btnBrowseDir = widgets.GetButton(t.Builder, "btn_browse_gen_dir")
	t.treeGenerate = widgets.GetTreeView(t.Builder, "tree_generate")
	t.listStore = widgets.GetListStore(t.Builder, "liststore_generate")
	t.entryChecksum = widgets.GetEntry(t.Builder, "entry_gen_checksum")
	t.btnSaveChk = widgets.GetButton(t.Builder, "btn_save_gen_checksum")
	t.cmbTxtAlgorithm = widgets.GetComboBoxText(t.Builder, "cmb_gen_algorithm")
	t.chkBtnFollowSymlinks = widgets.GetCheckButton(t.Builder, "chk_gen_follow_symlinks")
	t.chkBtnSortPaths = widgets.GetCheckButton(t.Builder, "chk_gen_sort_paths")
	t.chkBtnFlatPaths = widgets.GetCheckButton(t.Builder, "chk_gen_flat_paths")
	t.chkBtnSkipHidden = widgets.GetCheckButton(t.Builder, "chk_gen_skip_hidden")

	t.btnExclude = widgets.GetLinkButton(t.Builder, "btn_gen_exclude")
}

func (t *GenerateTab) getLabels() {
	t.labelProcessedV = widgets.GetLabel(t.Builder, "label_gen_processed_value")
	t.labelSkippedV = widgets.GetLabel(t.Builder, "label_gen_skipped_value")
	t.labelWithErrorsV = widgets.GetLabel(t.Builder, "label_gen_with_errors_value")
	t.labelPendingV = widgets.GetLabel(t.Builder, "label_gen_pending_value")
	t.labelTotalV = widgets.GetLabel(t.Builder, "label_gen_total_value")
	t.labelSpeedV = widgets.GetLabel(t.Builder, "label_gen_speed_value")
}

func (t *GenerateTab) setupHandlers() {
	t.btnBrowseDir.Connect("clicked", func() {
		path, _ := t.entryDir.GetText()
		if dir, ok := widgets.SelectDirectoryDialog(t.Window, "Select Source Directory", path); ok {
			t.entryDir.SetText(dir)

			extension := t.cmbTxtAlgorithm.GetActiveID()
			if t.chkBtnFlatPaths.GetActive() {
				t.entryChecksum.SetText(widgets.GenChecksumFilenameFlat(dir, extension))
			} else {
				t.entryChecksum.SetText(widgets.GenChecksumFilename(dir, extension))
			}
		}
	})

	onAlgorithmChanged := func() {
		extension := t.cmbTxtAlgorithm.GetActiveID()
		path, _ := t.entryChecksum.GetText()
		file := widgets.ChangeFileExtension(path, extension)
		t.entryChecksum.SetText(file)
	}

	syncAlgorithmFromPath := func() {
		checksumPath, _ := t.entryChecksum.GetText()
		if algo, err := algorithm.AlgorithmFromExtension(checksumPath); err == nil {
			t.cmbTxtAlgorithm.SetActiveID(algo.Extension())
		}
	}

	t.btnSaveChk.Connect("clicked", func() {
		checksumPath, _ := t.entryChecksum.GetText()
		if file, ok := widgets.SaveFileDialog(t.Window, "Save Checksum File", checksumPath, ""); ok {
			t.entryChecksum.SetText(file)

			if _, err := algorithm.AlgorithmFromExtension(file); err != nil {
				onAlgorithmChanged()
			}
		}
	})
	t.entryChecksum.Connect("changed", syncAlgorithmFromPath)
	t.entryChecksum.Connect("focus_out_event", func() {
		checksumPath, _ := t.entryChecksum.GetText()
		if _, err := algorithm.AlgorithmFromExtension(checksumPath); err != nil {
			onAlgorithmChanged()
		}
	})
	t.btnStart.Connect("clicked", t.onStart)
	t.btnStop.Connect("clicked", t.onStop)
	t.cmbTxtAlgorithm.Connect("changed", onAlgorithmChanged)
	t.entryDir.Connect("changed", func() {
		t.excludeRules = nil
		t.updateExcludeLabel()
	})
	t.chkBtnFollowSymlinks.Connect("toggled", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save generate settings", err)
		}
	})
	t.chkBtnSortPaths.Connect("toggled", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save generate settings", err)
		}
	})
	t.chkBtnFlatPaths.Connect("toggled", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save generate settings", err)
		}

		t.updateChecksumPathForFlatMode()
	})
	t.chkBtnSkipHidden.Connect("toggled", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save generate settings", err)
		}
	})
	t.cmbTxtAlgorithm.Connect("changed", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save generate settings", err)
		}
	})
	t.setupContextMenu()
	t.SetupColumnHandlers(t.treeGenerate, func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save generate settings", err)
		}
	})
	t.setupExcludeHandlers()
}

func (t *GenerateTab) onStart() {
	inputDir, _ := t.entryDir.GetText()
	outputFile, _ := t.entryChecksum.GetText()

	if !t.validateInputs(inputDir, outputFile) {
		return
	}

	inputDir = filepath.Clean(inputDir)
	outputFile = filepath.Clean(outputFile)

	if !t.confirmOverwriteIfNeeded(outputFile) {
		return
	}

	// Widget reads must complete before the goroutine launches: GTK access off the main thread is UB,
	// and excludeRules is reassigned by GTK signal handlers without synchronization, so the snapshot
	// below also dodges the slice-header race inside crash.Go.
	flatPaths := t.chkBtnFlatPaths.GetActive()
	followSymlinks := t.chkBtnFollowSymlinks.GetActive()
	sortPaths := t.chkBtnSortPaths.GetActive()
	skipHidden := t.chkBtnSkipHidden.GetActive()
	excludeRules := append([]exclude.Rule(nil), t.excludeRules...)

	lastStats := result.NewGeneratorStats()
	currentIdx := int64(0)

	t.activateStopState()

	// Wg.Add(1) immediately after the synchronous dialog accept so a shutdown during the async
	// setup below still waits for the goroutine and never leaks a started RunStream.
	t.Wg.Add(1)

	crash.Go("generateTab.onStart", func() {
		defer t.Wg.Done()
		defer t.CancelOperation()

		ctx, cancel := context.WithCancel(t.Ctx)
		t.SetCancel(cancel)

		algo, err := algorithm.AlgorithmFromExtension(outputFile)
		if err != nil {
			widgets.IdleAdd(t.Window, func() {
				t.setStartState()
				widgets.ShowError(t.Window, "Generation Error",
					fmt.Sprintf("Failed to resolve algorithm: %v", err))
			})

			return
		}

		var dirPrefix string

		if !flatPaths {
			var err error

			dirPrefix, err = walk.GetPrefixForFilesInChecksum(inputDir, outputFile)
			if err != nil {
				widgets.IdleAdd(t.Window, func() {
					t.setStartState()
					widgets.ShowError(t.Window, "Generation Error", fmt.Sprintf("Failed to get prefix: %v", err))
				})

				return
			}
		}

		cfg := generate.GenerateStreamingConfig{
			InputDir:            inputDir,
			OutputFile:          outputFile,
			Algorithm:           algo,
			DirPrefix:           dirPrefix,
			FollowSymbolicLinks: followSymlinks,
			SortPaths:           sortPaths,
			SkipHidden:          skipHidden,
			ExcludeMatcher:      exclude.NewMatcher(excludeRules),
		}

		results, err := generate.GenerateChecksumsStreamingToFile(ctx, cfg)
		if err != nil {
			widgets.IdleAdd(t.Window, func() {
				t.setStartState()
				widgets.ShowError(t.Window, "Generation Error", fmt.Sprintf("Failed to start generation: %v", err))
			})

			return
		}

		log.Info().
			Str("input_dir", inputDir).
			Str("output_file", outputFile).
			Str("algorithm", cfg.Algorithm.String()).
			Str("dir_prefix", cfg.DirPrefix).
			Bool("follow_symbolic_links", cfg.FollowSymbolicLinks).
			Bool("sort_paths", cfg.SortPaths).
			Bool("skip_hidden", cfg.SkipHidden).
			Bool("flat_paths", flatPaths).
			Strs("exclude", exclude.RuleStrings(excludeRules)).
			Msg("Starting generation")

		appendRows := func(items []generate.GenerateStreamingResult) {
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

					note := r.Result.Note
					if note != "" {
						note = errs.NormalizeText(note)
					} else if r.Result.Err != nil {
						note = errs.UnwrapAndNormalize(r.Result.Err)
					}

					vals[0] = currentIdx
					vals[1] = r.Result.Status.String()
					vals[2] = r.Result.RelPath
					vals[3] = bytesize.New(float64(r.Result.ReadBytes)).String()
					vals[4] = r.Result.Hash
					vals[5] = note
					vals[6] = r.Result.FullPath
					vals[7] = r.Result.Status.Color()
					vals[8] = r.Result.ReadBytes
					vals[9] = r.Result.Status.Priority()

					if err := t.listStore.InsertWithValues(nil, -1, cols, vals); err != nil {
						widgets.MustWidget("ListStore", "GenerateTab.appendRows", err)
					}

					lastStats = r.Stats
				}

				t.updateCounts(lastStats)
			})
		}

		widgets.RunStream(results, widgets.StreamBatchConfig[generate.GenerateStreamingResult]{
			FlushSize:     widgets.StreamFlushSize,
			FlushInterval: widgets.StreamFlushInterval,
			IsProgress:    func(r generate.GenerateStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r generate.GenerateStreamingResult) error { return r.Err },
			OnProgress: func(r generate.GenerateStreamingResult) {
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
							log.Warn().Msg("Generation canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to generate checksums")
							widgets.ShowError(t.Window, "Generation Error",
								fmt.Sprintf("Failed to generate checksums: %v", hasError))
						}
					} else {
						log.Info().
							Int("processed", stats.Processed).
							Int("skipped", stats.Skipped).
							Int("with_errors", stats.WithErrors).
							Int("pending", stats.Pending()).
							Int("total_files", stats.TotalFiles).
							Msg("Generation stats")
						log.Info().
							Str("output_file", outputFile).
							Msg("Generation completed")
					}

					t.setStartState()

					if hasError == nil && stats.WithErrors == 0 {
						base.SetFinalLabel(t.labelProcessedV, stats.Processed, result.GenSuccess.Color())
					}
				})
			},
		})
	})
}

func (t *GenerateTab) onStop() {
	t.CancelOperation()
}

func (t *GenerateTab) activateStopState() {
	lastStats := result.NewGeneratorStats()

	t.listStore.Clear()
	t.updateCounts(lastStats)
	t.updateLiveStatus(lastStats)
	t.SuspendSort(t.treeGenerate)
	widgets.ResetColumnWidths(t.treeGenerate)

	t.btnStart.SetVisible(false)
	t.btnStop.SetVisible(true)
	t.progressTracker.ActivateStopState()
	t.btnBrowseDir.SetSensitive(false)
	t.btnSaveChk.SetSensitive(false)
	t.entryDir.SetSensitive(false)
	t.entryChecksum.SetSensitive(false)
	t.cmbTxtAlgorithm.SetSensitive(false)
	t.chkBtnFollowSymlinks.SetSensitive(false)
	t.chkBtnSortPaths.SetSensitive(false)
	t.chkBtnFlatPaths.SetSensitive(false)
	t.chkBtnSkipHidden.SetSensitive(false)
	t.btnExclude.SetSensitive(false)
}

func (t *GenerateTab) setStartState() {
	t.btnStart.SetVisible(true)
	t.btnStop.SetVisible(false)
	t.progressTracker.SetStartState()
	t.btnBrowseDir.SetSensitive(true)
	t.btnSaveChk.SetSensitive(true)
	t.entryDir.SetSensitive(true)
	t.entryChecksum.SetSensitive(true)
	t.cmbTxtAlgorithm.SetSensitive(true)
	t.chkBtnFollowSymlinks.SetSensitive(true)
	t.chkBtnSortPaths.SetSensitive(true)
	t.chkBtnFlatPaths.SetSensitive(true)
	t.chkBtnSkipHidden.SetSensitive(true)
	t.btnExclude.SetSensitive(true)

	t.RestoreSort()
	widgets.ScrollListToTop(t.treeGenerate)
}

// updateCounts renders the counters and aggregate bar from stats taken after a table insert,
// so they never run ahead of the visible rows. updateLiveStatus renders the fields driven by
// producer ticks (speed, current file, per-file bar), which move between inserts.
func (t *GenerateTab) updateCounts(stats result.GeneratorStats) {
	t.labelProcessedV.SetText(fmt.Sprintf("%d files", stats.Processed))
	base.SetStatLabel(t.labelSkippedV, stats.Skipped, result.GenSkipped.Color())
	base.SetStatLabel(t.labelWithErrorsV, stats.WithErrors, result.GenFailed.Color())
	t.labelPendingV.SetText(fmt.Sprintf("%d files", stats.Pending()))
	t.labelTotalV.SetText(fmt.Sprintf("%d files", stats.TotalFiles))
	t.progressTracker.UpdateTotalProgress(stats.TotalProgress())
}

func (t *GenerateTab) updateLiveStatus(stats result.GeneratorStats) {
	t.labelSpeedV.SetText(bytesize.New(stats.Speed).String() + "/s")
	t.progressTracker.UpdateCurrentFile(stats.CurrentFileOrStatus)
	t.progressTracker.UpdateFileProgress(stats.FileHashingProgress)
}

func (t *GenerateTab) confirmOverwriteIfNeeded(outputFile string) bool {
	if err := fs.ShouldOverwrite(outputFile, false); err != nil {
		if errors.Is(err, fs.ErrRefuseOverwrite) {
			return widgets.ShowConfirmOverwriteDialog(t.Window, outputFile)
		}

		widgets.ShowError(t.Window, "Invalid Output Path", err.Error())

		return false
	}

	return true
}

func (t *GenerateTab) saveSettings() error {
	if !widgets.IsAlive(t.Window) {
		return nil
	}

	t.Settings.Generate.FollowSymbolicLinks = t.chkBtnFollowSymlinks.GetActive()
	t.Settings.Generate.SortPaths = t.chkBtnSortPaths.GetActive()
	t.Settings.Generate.FlatPaths = t.chkBtnFlatPaths.GetActive()
	t.Settings.Generate.SkipHidden = t.chkBtnSkipHidden.GetActive()
	t.Settings.Generate.Algorithm = t.cmbTxtAlgorithm.GetActiveID()
	t.Settings.Generate.ColumnOrder = t.ColumnConfig.GetColumnOrder(t.treeGenerate)

	// While a bulk run has sorting suspended the indicators are hidden, so GetSortState would
	// report "" and overwrite the persisted column; keep the saved value in that window.
	if !t.SortSuspended() {
		sortColumn, sortOrder := t.ColumnConfig.GetSortState(t.treeGenerate)

		t.Settings.Generate.SortColumn = sortColumn
		if sortOrder == gtk.SORT_DESCENDING {
			t.Settings.Generate.SortOrder = settings.SortOrderDesc
		} else {
			t.Settings.Generate.SortOrder = settings.SortOrderAsc
		}
	}

	return t.Settings.AsyncSave()
}

func (t *GenerateTab) applySettingsToUI() {
	t.chkBtnFollowSymlinks.SetActive(t.Settings.Generate.FollowSymbolicLinks)
	t.chkBtnSortPaths.SetActive(t.Settings.Generate.SortPaths)
	t.chkBtnFlatPaths.SetActive(t.Settings.Generate.FlatPaths)
	t.chkBtnSkipHidden.SetActive(t.Settings.Generate.SkipHidden)
	t.cmbTxtAlgorithm.SetActiveID(t.Settings.Generate.Algorithm)
	t.ColumnConfig.ApplyColumnOrder(t.treeGenerate, t.Settings.Generate.ColumnOrder)
	t.ApplySortOrder(t.treeGenerate, t.Settings.Generate.SortColumn, t.Settings.Generate.SortOrder)
}

func (t *GenerateTab) setupContextMenu() {
	columnLabels := []string{"index", "status", "path", "size", "hash", "note"}
	t.contextMenuProvider.CreateMenuWithReveal(6, columnLabels, t.revealSelectedFile)
	t.contextMenuProvider.ConnectRightClick(func() {
		t.contextMenuProvider.ShowMenu()
	})
}

func (t *GenerateTab) revealSelectedFile(fullPath string) {
	crash.Go("generateTab.revealSelectedFile", func() {
		if err := reveal.Reveal(t.Ctx, fullPath); err != nil {
			widgets.IdleAdd(t.Window, func() {
				widgets.ShowError(t.Window, "Reveal Error",
					fmt.Sprintf("Failed to open file manager:\n%v", err))
			})

			return
		}
	})
}

func (t *GenerateTab) validateInputs(inputDir, outputFile string) bool {
	if inputDir == "" {
		widgets.ShowError(t.Window, "No Source Directory", "Please select a source directory.")

		return false
	}

	if err := generate.ValidateInputDir(inputDir); err != nil {
		widgets.ShowError(t.Window, "Invalid Source Path",
			fmt.Sprintf("Source directory is unavailable:\n%s", err))

		return false
	}

	if outputFile == "" {
		widgets.ShowError(t.Window, "No Checksum File", "Please specify a checksum file path.")

		return false
	}

	// Validate the extension on the cleaned path, the same one the run uses: the CLI cleans
	// first, so a trailing separator must not turn a valid extension into "unsupported".
	outputFile = filepath.Clean(outputFile)

	if _, err := algorithm.AlgorithmFromExtension(outputFile); err != nil {
		widgets.ShowError(t.Window, "Unsupported Extension",
			"Checksum file has an unsupported extension.")

		return false
	}

	return true
}

func (t *GenerateTab) setupExcludeCSS() {
	cssProvider, err := gtk.CssProviderNew()
	if err != nil {
		widgets.MustWidget("CssProvider", "GenerateTab.setupExcludeCSS", err)
	}

	css := `
		.exclude-link {
			padding: 0;
			margin: 0;
		}
	`
	if err := cssProvider.LoadFromData(css); err != nil {
		widgets.MustWidget("CssProvider.LoadFromData", "GenerateTab.setupExcludeCSS", err)
	}

	screen, err := t.btnExclude.GetScreen()
	if err != nil {
		widgets.MustWidget("Screen", "GenerateTab.setupExcludeCSS", err)
	}

	gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

func (t *GenerateTab) setupExcludeHandlers() {
	t.btnExclude.Connect("activate-link", func(_ *gtk.LinkButton) bool {
		inputDir, _ := t.entryDir.GetText()
		if inputDir == "" {
			widgets.ShowError(t.Window, "No Source Directory", "Please select a source directory first.")

			return true
		}

		if err := generate.ValidateInputDir(inputDir); err != nil {
			widgets.ShowError(t.Window, "Invalid Source Path",
				fmt.Sprintf("Source directory is unavailable:\n%s", err))

			return true
		}

		checksumPath, _ := t.entryChecksum.GetText()

		dlg, err := widgets.NewExcludeDialog(
			t.Window,
			"Exclude Files",
			inputDir,
			checksumPath,
			t.excludeRules,
			t.chkBtnSkipHidden.GetActive(),
			t.Settings.Generate.ExcludeDialog.Width,
			t.Settings.Generate.ExcludeDialog.Height,
		)
		if err != nil {
			widgets.ShowError(t.Window, "Exclude Files",
				fmt.Sprintf("Failed to open exclude dialog:\n%v", err))

			return true
		}
		defer dlg.Destroy()

		excluded, ok := dlg.Run()

		w, h := dlg.GetSize()
		t.Settings.Generate.ExcludeDialog.Width = w
		t.Settings.Generate.ExcludeDialog.Height = h

		if err := t.Settings.AsyncSave(); err != nil {
			t.LogError("save exclude dialog state", err)
		}

		if !ok {
			return true
		}

		t.excludeRules = excluded
		t.updateExcludeLabel()

		return true
	})
}

func (t *GenerateTab) updateExcludeLabel() {
	t.btnExclude.SetLabel(fmt.Sprintf("Exclude (%d)", len(t.excludeRules)))
}

func (t *GenerateTab) updateChecksumPathForFlatMode() {
	inputDir, _ := t.entryDir.GetText()
	if inputDir == "" {
		return
	}

	extension := t.cmbTxtAlgorithm.GetActiveID()
	checksumPath, _ := t.entryChecksum.GetText()

	var expected, opposite string
	if t.chkBtnFlatPaths.GetActive() {
		expected = widgets.GenChecksumFilenameFlat(inputDir, extension)
		opposite = widgets.GenChecksumFilename(inputDir, extension)
	} else {
		expected = widgets.GenChecksumFilename(inputDir, extension)
		opposite = widgets.GenChecksumFilenameFlat(inputDir, extension)
	}

	if checksumPath == "" || checksumPath == opposite {
		t.entryChecksum.SetText(expected)
	}
}

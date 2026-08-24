package tabs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
	"github.com/ostapkonst/HashVerifier/internal/service/generate"
)

type GenerateTab struct {
	*TabBase
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
	contextMenuProvider  *widgets.ContextMenuProvider
	progressTracker      *ProgressTracker
	labelProcessedV      *gtk.Label
	labelSkippedV        *gtk.Label
	labelWithErrorsV     *gtk.Label
	labelPendingV        *gtk.Label
	labelSpeedV          *gtk.Label

	btnExclude      *gtk.LinkButton
	excludeRelPaths []string
}

func NewGenerateTab(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings) *GenerateTab {
	tab := &GenerateTab{
		TabBase: NewTabBase(ctx, builder, window, settings, NewGenerateColumnConfig()),
	}
	tab.getWidgets()
	tab.getLabels()
	tab.progressTracker = NewProgressTracker(
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

func (t *GenerateTab) Fill(path string) error {
	if t.IsBusy() {
		return ErrTabBusy
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

	t.btnExclude = widgets.GetLinkButton(t.Builder, "btn_gen_exclude")
}

func (t *GenerateTab) getLabels() {
	t.labelProcessedV = widgets.GetLabel(t.Builder, "label_gen_processed_value")
	t.labelSkippedV = widgets.GetLabel(t.Builder, "label_gen_skipped_value")
	t.labelWithErrorsV = widgets.GetLabel(t.Builder, "label_gen_with_errors_value")
	t.labelPendingV = widgets.GetLabel(t.Builder, "label_gen_pending_value")
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
		t.excludeRelPaths = nil
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

	lastStats := result.NewGeneratorStats()
	currentIdx := int64(0)

	t.activateStopState()

	ctx, cancel := context.WithCancel(t.Ctx)
	t.Cancel = cancel

	algo, _ := algorithm.AlgorithmFromExtension(outputFile)

	flatPaths := t.chkBtnFlatPaths.GetActive()

	var dirPrefix string

	if !flatPaths {
		var err error

		dirPrefix, err = walk.GetPrefixForFilesInChecksum(inputDir, outputFile)
		if err != nil {
			t.CancelOperation()
			t.setStartState()

			widgets.ShowError(t.Window, "Generation Error", fmt.Sprintf("Failed to get prefix: %v", err))

			return
		}
	}

	cfg := generate.GenerateStreamingConfig{
		InputDir:            inputDir,
		OutputFile:          outputFile,
		Algorithm:           algo,
		DirPrefix:           dirPrefix,
		FollowSymbolicLinks: t.chkBtnFollowSymlinks.GetActive(),
		SortPaths:           t.chkBtnSortPaths.GetActive(),
		FlatPaths:           flatPaths,
		ExcludeMatcher:      exclude.NewMatcher(t.excludeRelPaths),
	}

	results, err := generate.GenerateChecksumsStreamingToFile(ctx, cfg)
	if err != nil {
		t.CancelOperation()
		t.setStartState()

		widgets.ShowError(t.Window, "Generation Error", fmt.Sprintf("Failed to start generation: %v", err))

		return
	}

	log.Info().
		Str("input_dir", inputDir).
		Str("output_file", outputFile).
		Str("algorithm", cfg.Algorithm.String()).
		Str("dir_prefix", cfg.DirPrefix).
		Bool("follow_symbolic_links", cfg.FollowSymbolicLinks).
		Bool("sort_paths", cfg.SortPaths).
		Bool("flat_paths", cfg.FlatPaths).
		Strs("exclude", t.excludeRelPaths).
		Msg("Starting generation")

	t.Wg.Add(1)

	appendRows := func(items []generate.GenerateStreamingResult) {
		glib.IdleAdd(func() {
			for i := range items {
				r := items[i]
				currentIdx += 1
				iter := t.listStore.Append()

				_ = t.listStore.SetValue(iter, 0, currentIdx)
				_ = t.listStore.SetValue(iter, 1, r.Result.Status.String())
				_ = t.listStore.SetValue(iter, 2, r.Result.RelPath)
				_ = t.listStore.SetValue(iter, 3, bytesize.New(float64(r.Result.ReadBytes)).String())

				_ = t.listStore.SetValue(iter, 4, r.Result.Hash)
				if r.Result.Err != nil {
					_ = t.listStore.SetValue(iter, 5, errs.UnwrapAndNormalize(r.Result.Err))
				}

				_ = t.listStore.SetValue(iter, 6, r.Result.FullPath)
				_ = t.listStore.SetValue(iter, 7, r.Result.Status.Color())
				_ = t.listStore.SetValue(iter, 8, r.Result.ReadBytes)
				_ = t.listStore.SetValue(iter, 9, r.Result.Status.Priority())
				lastStats = r.Stats
			}

			t.updateStats(lastStats)
		})
	}

	go func() {
		defer t.Wg.Done()

		widgets.RunStream(results, widgets.StreamBatchConfig[generate.GenerateStreamingResult]{
			FlushSize:     200,
			FlushInterval: 150 * time.Millisecond,
			IsProgress:    func(r generate.GenerateStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r generate.GenerateStreamingResult) error { return r.Err },
			OnProgress: func(r generate.GenerateStreamingResult) {
				glib.IdleAdd(func() {
					lastStats = r.Stats
					t.updateStats(lastStats)
				})
			},
			OnBatch: appendRows,
			OnFinish: func(hasError error) {
				glib.IdleAdd(func() {
					if hasError != nil {
						if errors.Is(hasError, context.Canceled) {
							log.Warn().Msg("Generation canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to generate checksums")
							widgets.ShowError(t.Window, "Generation Error",
								fmt.Sprintf("Failed to generate checksums: %v", hasError))
						}
					} else {
						log.Info().
							Int("processed", lastStats.Processed).
							Int("skipped", lastStats.Skipped).
							Int("with_errors", lastStats.WithErrors).
							Int("pending", lastStats.Pending()).
							Int("total_files", lastStats.TotalFiles).
							Msg("Generation stats")
						log.Info().
							Str("output_file", outputFile).
							Msg("Generation completed")
					}

					t.CancelOperation()
					t.setStartState()

					color := result.GenFailed.Color()
					if hasError == nil && lastStats.WithErrors == 0 && lastStats.Pending() == 0 {
						color = result.GenSuccess.Color()
					}

					setFinalLabel(
						t.labelProcessedV,
						lastStats.Processed, lastStats.TotalFiles,
						lastStats.Pending(),
						color,
					)
				})
			},
		})
	}()
}

func (t *GenerateTab) onStop() {
	t.CancelOperation()
}

func (t *GenerateTab) activateStopState() {
	lastStats := result.NewGeneratorStats()

	t.listStore.Clear()
	t.updateStats(lastStats)

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
	t.btnExclude.SetSensitive(true)
}

func (t *GenerateTab) updateStats(stats result.GeneratorStats) {
	t.labelProcessedV.SetText(fmt.Sprintf("%d of %d files", stats.Processed, stats.TotalFiles))
	setStatLabel(t.labelSkippedV, stats.Skipped, stats.TotalFiles, result.GenSkipped.Color())
	setStatLabel(t.labelWithErrorsV, stats.WithErrors, stats.TotalFiles, result.GenFailed.Color())
	t.labelPendingV.SetText(fmt.Sprintf("%d of %d files", stats.Pending(), stats.TotalFiles))
	t.labelSpeedV.SetText(bytesize.New(stats.Speed).String() + "/s")
	t.progressTracker.UpdateCurrentFile(stats.CurrentFileOrStatus)
	t.progressTracker.UpdateTotalProgress(stats.TotalProgress())
	t.progressTracker.UpdateFileProgress(stats.FileHashingProgress)
}

func (t *GenerateTab) confirmOverwriteIfNeeded(outputFile string) bool {
	absPath, err := filepath.Abs(outputFile)
	if err != nil {
		absPath = outputFile
	}

	if err := fs.ShouldOverwrite(absPath, false); err != nil {
		if errors.Is(err, fs.ErrRefuseOverwrite) {
			return widgets.ShowConfirmOverwriteDialog(t.Window, absPath)
		}

		widgets.ShowError(t.Window, "Invalid Output Path", err.Error())

		return false
	}

	return true
}

func (t *GenerateTab) saveSettings() error {
	if t.Window.InDestruction() {
		return nil
	}

	t.Settings.Generate.FollowSymbolicLinks = t.chkBtnFollowSymlinks.GetActive()
	t.Settings.Generate.SortPaths = t.chkBtnSortPaths.GetActive()
	t.Settings.Generate.FlatPaths = t.chkBtnFlatPaths.GetActive()
	t.Settings.Generate.Algorithm = t.cmbTxtAlgorithm.GetActiveID()
	t.Settings.Generate.ColumnOrder = t.ColumnConfig.GetColumnOrder(t.treeGenerate)
	sortColumn, sortOrder := t.ColumnConfig.GetSortState(t.treeGenerate)

	t.Settings.Generate.SortColumn = sortColumn
	if sortOrder == gtk.SORT_DESCENDING {
		t.Settings.Generate.SortOrder = settings.SortOrderDesc
	} else {
		t.Settings.Generate.SortOrder = settings.SortOrderAsc
	}

	return t.Settings.Save()
}

func (t *GenerateTab) applySettingsToUI() {
	t.chkBtnFollowSymlinks.SetActive(t.Settings.Generate.FollowSymbolicLinks)
	t.chkBtnSortPaths.SetActive(t.Settings.Generate.SortPaths)
	t.chkBtnFlatPaths.SetActive(t.Settings.Generate.FlatPaths)
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
	go func() {
		if err := platform.RevealFile(t.Ctx, fullPath); err != nil {
			glib.IdleAdd(func() {
				widgets.ShowError(t.Window, "Reveal Error",
					fmt.Sprintf("Failed to open file manager:\n%v", err))
			})

			return
		}
	}()
}

func (t *GenerateTab) validateInputs(inputDir, outputFile string) bool {
	if inputDir == "" {
		widgets.ShowError(t.Window, "No Source Directory", "Please select a source directory.")

		return false
	}

	info, err := os.Stat(inputDir)
	if err != nil {
		widgets.ShowError(t.Window, "Source Directory Not Found",
			fmt.Sprintf("Source directory does not exist:\n%s", inputDir))

		return false
	}

	if !info.IsDir() {
		widgets.ShowError(t.Window, "Invalid Source Path",
			fmt.Sprintf("Source path is not a directory:\n%s", inputDir))

		return false
	}

	if outputFile == "" {
		widgets.ShowError(t.Window, "No Checksum File", "Please specify a checksum file path.")

		return false
	}

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
		return
	}

	css := `
		.exclude-link {
			padding: 0;
			margin: 0;
		}
	`
	if err := cssProvider.LoadFromData(css); err != nil {
		return
	}

	screen, err := t.btnExclude.GetScreen()
	if err != nil {
		return
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

		info, err := os.Stat(inputDir)
		if err != nil {
			widgets.ShowError(t.Window, "Source Directory Not Found",
				fmt.Sprintf("Source directory does not exist:\n%s", inputDir))

			return true
		}

		if !info.IsDir() {
			widgets.ShowError(t.Window, "Invalid Source Path",
				fmt.Sprintf("Source path is not a directory:\n%s", inputDir))

			return true
		}

		checksumPath, _ := t.entryChecksum.GetText()

		dlg := widgets.NewExcludeDialog(
			t.Window,
			"Exclude Files from Generation",
			inputDir,
			checksumPath,
			t.excludeRelPaths,
			t.Settings.Generate.ExcludeDialog.Width,
			t.Settings.Generate.ExcludeDialog.Height,
		)
		if dlg == nil {
			return true
		}

		defer dlg.Destroy()

		excluded, ok := dlg.Run()

		w, h := dlg.GetSize()
		t.Settings.Generate.ExcludeDialog.Width = w
		t.Settings.Generate.ExcludeDialog.Height = h

		if err := t.Settings.Save(); err != nil {
			t.LogError("save exclude dialog state", err)
		}

		if !ok {
			return true
		}

		t.excludeRelPaths = excluded
		t.updateExcludeLabel()

		return true
	})
}

func (t *GenerateTab) updateExcludeLabel() {
	t.btnExclude.SetLabel(fmt.Sprintf("Exclude (%d)", len(t.excludeRelPaths)))
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

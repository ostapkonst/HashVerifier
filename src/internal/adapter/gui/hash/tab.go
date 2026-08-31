// Package hash implements the Hash GUI tab: single-file hash with per-algorithm toggles, export, and search.
package hash

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/appmeta"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
	"github.com/ostapkonst/HashVerifier/internal/service/hash"
)

// HashTab embeds base.TabBase and adds GTK widgets and callbacks for the Hash use-case (single file, per-algorithm toggles).
type HashTab struct {
	*base.TabBase
	entryFile           *gtk.Entry
	btnStart            *gtk.Button
	btnStop             *gtk.Button
	btnBrowseFile       *gtk.Button
	treeHash            *gtk.TreeView
	listStore           *gtk.ListStore
	chkHashOnOpen       *gtk.CheckButton
	frameHashProgress   *gtk.Frame
	progressBar         *gtk.ProgressBar
	cellRendererToggle  *gtk.CellRendererToggle
	contextMenuProvider *widgets.ContextMenuProvider
	searchEntry         *gtk.SearchEntry
	hashedFilePath      string
	algoByExt           map[string]algorithm.Algorithm
}

// NewHashTab initializes the Hash tab's widgets, restores persisted settings, and connects every handler.
func NewHashTab(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings) *HashTab {
	tab := &HashTab{
		TabBase: base.NewTabBase(ctx, builder, window, settings, nil),
	}
	tab.getWidgets()
	tab.contextMenuProvider = widgets.NewContextMenuProvider(tab.treeHash, tab.listStore)
	tab.applySettingsToUI()
	tab.setupSearchCSS()
	tab.setStartState()
	tab.setupHandlers()

	return tab
}

func (t *HashTab) getWidgets() {
	t.entryFile = widgets.GetEntry(t.Builder, "entry_hash_file")
	t.btnStart = widgets.GetButton(t.Builder, "btn_start_hashing")
	t.btnStop = widgets.GetButton(t.Builder, "btn_stop_hashing")
	t.btnBrowseFile = widgets.GetButton(t.Builder, "btn_browse_hash_file")
	t.treeHash = widgets.GetTreeView(t.Builder, "tree_hash")
	t.listStore = widgets.GetListStore(t.Builder, "liststore_hash")
	t.chkHashOnOpen = widgets.GetCheckButton(t.Builder, "chk_hash_hashing_on_open")
	t.frameHashProgress = widgets.GetFrame(t.Builder, "grid_hash_progress")
	t.progressBar = widgets.GetProgressBar(t.Builder, "progress_hash")
	t.cellRendererToggle = widgets.GetCellRendererToggle(t.Builder, "cell_renderer_hash_toggle")
	t.searchEntry = widgets.GetSearchEntry(t.Builder, "search_hash_in_table")
}

func (t *HashTab) populateAlgorithmTable() {
	t.listStore.Clear()

	t.algoByExt = make(map[string]algorithm.Algorithm, len(algorithm.SupportedAlgorithms))

	enabledAlgos := make(map[string]bool)

	for _, algoExt := range t.Settings.Hash.Algorithms {
		if a, err := algorithm.AlgorithmFromExtension(algoExt); err == nil {
			enabledAlgos[a.Extension()] = true
		}
	}

	for _, a := range algorithm.SupportedAlgorithms {
		iter := t.listStore.Append()
		if err := t.listStore.SetValue(iter, 0, a.DisplayName()); err != nil {
			widgets.MustWidget("ListStore", "HashTab.populateAlgorithms", err)
		}

		if err := t.listStore.SetValue(iter, 1, ""); err != nil {
			widgets.MustWidget("ListStore", "HashTab.populateAlgorithms", err)
		}

		if err := t.listStore.SetValue(iter, 2, a.Extension()); err != nil {
			widgets.MustWidget("ListStore", "HashTab.populateAlgorithms", err)
		}

		if err := t.listStore.SetValue(iter, 3, enabledAlgos[a.Extension()]); err != nil {
			widgets.MustWidget("ListStore", "HashTab.populateAlgorithms", err)
		}

		t.algoByExt[a.Extension()] = a
	}
}

func (t *HashTab) setupHandlers() {
	t.btnBrowseFile.Connect("clicked", func() {
		path, _ := t.entryFile.GetText()
		if file, ok := widgets.OpenAnyFileDialog(t.Window, "Select File to Hash", path); ok {
			t.entryFile.SetText(file)

			if t.chkHashOnOpen.GetActive() {
				t.onStart()
			}
		}
	})
	t.btnStart.Connect("clicked", t.onStart)
	t.btnStop.Connect("clicked", t.onStop)
	t.setupToggleHandler()
	t.chkHashOnOpen.Connect("toggled", func() {
		if err := t.saveSettings(); err != nil {
			t.LogError("save hash settings", err)
		}
	})
	t.setupContextMenu()
	t.searchEntry.Connect("search-changed", func() {
		t.applySearchHighlighting()
	})
	t.searchEntry.Connect("stop-search", func() {
		t.applySearchHighlighting()
	})
}

func (t *HashTab) applySearchHighlighting() {
	query, _ := t.searchEntry.GetText()
	query = strings.TrimSpace(query)

	styleContext, _ := t.searchEntry.GetStyleContext()
	styleContext.RemoveClass("search-found")
	styleContext.RemoveClass("search-not-found")

	if query == "" {
		t.searchEntry.QueueDraw()
		return
	}

	var hashes []string

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		hash, ok := widgets.ListStoreString(t.listStore, iter, 1)
		if ok && hash != "" {
			hashes = append(hashes, hash)
		}

		return true
	})

	if len(hashes) == 0 {
		t.searchEntry.QueueDraw()
		return
	}

	found := false

	for _, hash := range hashes {
		if strings.EqualFold(hash, query) {
			found = true
			break
		}
	}

	if found {
		styleContext.AddClass("search-found")
	} else {
		styleContext.AddClass("search-not-found")
	}

	t.searchEntry.QueueDraw()
}

func (t *HashTab) forEachRow(fn func(iter *gtk.TreeIter) bool) {
	iter, ok := t.listStore.GetIterFirst()
	if !ok {
		return
	}

	for fn(iter) {
		if !t.listStore.IterNext(iter) {
			break
		}
	}
}

func (t *HashTab) setupContextMenu() {
	columnLabels := []string{"algorithm", "hashsum"}
	t.contextMenuProvider.CreateMenuWithExportItem(
		"Export",
		t.exportSelectedHash,
		[]int{0, 1},
		columnLabels,
	)
	t.contextMenuProvider.ConnectRightClick(func() {
		t.contextMenuProvider.ShowMenu()
	})
}

func (t *HashTab) exportSelectedHash() {
	selection, err := t.treeHash.GetSelection()
	if err != nil {
		widgets.MustWidget("TreeView", "HashTab.exportSelectedHash:GetSelection", err)
	}

	_, iter, ok := selection.GetSelected()
	if !ok {
		return
	}

	hashStr, ok := widgets.ListStoreString(t.listStore, iter, 1)
	if !ok {
		return
	}

	extStr, ok := widgets.ListStoreString(t.listStore, iter, 2)
	if !ok {
		return
	}

	if hashStr == "" {
		widgets.ShowError(t.Window, "Export Hash", "Hash has not been calculated yet.")

		return
	}

	sourcePath := t.hashedFilePath
	if sourcePath == "" {
		widgets.ShowError(t.Window, "Export Hash", "Source file path is not available.")

		return
	}

	if err := hash.ValidateFilePath(sourcePath); err != nil {
		widgets.ShowError(t.Window, "Export Hash",
			fmt.Sprintf("Source file is unavailable:\n%s", sourcePath))

		return
	}

	algoType, ok := t.algoByExt[extStr]
	if !ok {
		widgets.ShowError(t.Window, "Export Hash",
			fmt.Sprintf("Unsupported extension %s.", extStr))

		return
	}

	defaultFolder := filepath.Dir(sourcePath)
	defaultName := "checksums" + extStr

	savePath, ok := widgets.SaveFileDialog(
		t.Window,
		"Export Hash as Checksum",
		filepath.Join(defaultFolder, defaultName),
		extStr,
	)
	if !ok {
		return
	}

	if err := fs.ShouldOverwrite(savePath, false); err != nil {
		if errors.Is(err, fs.ErrRefuseOverwrite) {
			if !widgets.ShowConfirmOverwriteDialog(t.Window, savePath) {
				return
			}
		} else {
			widgets.ShowError(t.Window, "Export Hash", err.Error())

			return
		}
	}

	relPath := filepath.Base(sourcePath)

	line := walk.FormatLine(relPath, hashStr, algoType)

	content := appmeta.FormatExportedFile(line)

	if err := os.WriteFile(savePath, []byte(content), 0o644); err != nil {
		widgets.ShowError(t.Window, "Export Hash",
			fmt.Sprintf("Failed to write file:\n%s", err))

		return
	}

	log.Info().
		Str("file", savePath).
		Str("algorithm", algoType.String()).
		Str("hash", hashStr).
		Msg("Hash exported to checksum file")
}

func (t *HashTab) setupToggleHandler() {
	if t.cellRendererToggle == nil {
		return
	}

	t.cellRendererToggle.Connect("toggled", func(_ *gtk.CellRendererToggle, pathStr string) {
		path, err := gtk.TreePathNewFromString(pathStr)
		if err != nil {
			widgets.MustWidget("TreePath", "HashTab.cellRendererToggle:TreePathNewFromString", err)
		}

		t.toggleAlgorithmAtPath(path)

		if err := t.saveSettings(); err != nil {
			t.LogError("save hash settings", err)
		}
	})
}

func (t *HashTab) toggleAlgorithmAtPath(path *gtk.TreePath) {
	iter, err := t.listStore.GetIter(path)
	if err != nil {
		widgets.MustWidget("ListStore", "HashTab.toggleAlgorithmAtPath:GetIter", err)
	}

	currentState, ok := widgets.ListStoreBool(t.listStore, iter, 3)
	if !ok {
		return
	}

	if err := t.listStore.SetValue(iter, 3, !currentState); err != nil {
		widgets.MustWidget("ListStore", "HashTab.toggleAlgorithmAtPath", err)
	}
}

func (t *HashTab) getSelectedAlgorithms() []string {
	var selected []string

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		enabled, ok := widgets.ListStoreBool(t.listStore, iter, 3)
		if !ok || !enabled {
			return true
		}

		ext, ok := widgets.ListStoreString(t.listStore, iter, 2)
		if !ok {
			return true
		}

		selected = append(selected, ext)

		return true
	})

	return selected
}

func (t *HashTab) resolveSelectedAlgorithms() []algorithm.Algorithm {
	selectedExts := t.getSelectedAlgorithms()

	algos := make([]algorithm.Algorithm, 0, len(selectedExts))
	for _, ext := range selectedExts {
		if algo, ok := t.algoByExt[ext]; ok {
			algos = append(algos, algo)
		}
	}

	return algos
}

func algoNames(algos []algorithm.Algorithm) []string {
	names := make([]string, len(algos))
	for i, a := range algos {
		names[i] = a.String()
	}

	return names
}

// Fill sets the input-file field from path and triggers auto-start when HashOnOpen is enabled.
// Returns base.ErrTabBusy if the tab is currently running.
func (t *HashTab) Fill(path string) error {
	if t.IsBusy() {
		return base.ErrTabBusy
	}

	t.entryFile.SetText(path)

	if t.chkHashOnOpen.GetActive() {
		t.onStart()
	}

	return nil
}

func (t *HashTab) validateInputs(filePath string) bool {
	if filePath == "" {
		widgets.ShowError(t.Window, "No File Selected", "Please select a file to hash.")

		return false
	}

	info, err := os.Stat(filePath)
	if err != nil {
		widgets.ShowError(t.Window, "File Not Found",
			fmt.Sprintf("File does not exist:\n%s", filePath))

		return false
	}

	if info.IsDir() {
		widgets.ShowError(t.Window, "Invalid Path",
			fmt.Sprintf("Selected path is a directory:\n%s", filePath))

		return false
	}

	if !info.Mode().IsRegular() {
		widgets.ShowError(t.Window, "Invalid Path",
			fmt.Sprintf("Selected path is not a regular file:\n%s", filePath))

		return false
	}

	if len(t.getSelectedAlgorithms()) == 0 {
		widgets.ShowError(t.Window, "No Algorithms Selected",
			"Please select at least one algorithm.")

		return false
	}

	return true
}

func (t *HashTab) onStart() {
	filePath, _ := t.entryFile.GetText()

	if !t.validateInputs(filePath) {
		return
	}

	filePath = filepath.Clean(filePath)
	t.hashedFilePath = filePath

	selectedAlgos := t.resolveSelectedAlgorithms()

	t.activateStopState()

	ctx, cancel := context.WithCancel(t.Ctx)
	t.SetCancel(cancel)

	cfg := hash.HashConfig{
		FilePath:   filePath,
		Algorithms: selectedAlgos,
	}

	results, err := hash.HashFileStreaming(ctx, cfg)
	if err != nil {
		t.CancelOperation()
		t.setStartState()

		widgets.ShowError(t.Window, "Hashing Error", fmt.Sprintf("Failed to start hashing: %v", err))

		return
	}

	log.Info().
		Str("file", filePath).
		Strs("algorithms", algoNames(selectedAlgos)).
		Msg("Starting hashing")

	t.Wg.Add(1)

	go func() {
		defer t.Wg.Done()

		widgets.RunStream(results, widgets.StreamBatchConfig[hash.HashStreamingResult]{
			FlushSize:     200,
			FlushInterval: 150 * time.Millisecond,
			IsProgress:    func(r hash.HashStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r hash.HashStreamingResult) error { return r.Err },
			OnProgress: func(r hash.HashStreamingResult) {
				widgets.IdleAdd(t.Window, func() {
					t.updateStats(r.Progress)
				})
			},
			OnBatch: func(items []hash.HashStreamingResult) {
				widgets.IdleAdd(t.Window, func() {
					for _, r := range items {
						for algo, hash := range r.Result.Hashes {
							t.updateHashForAlgorithm(algo, hash)
						}
					}
				})
			},
			OnFinish: func(hasError error) {
				widgets.IdleAdd(t.Window, func() {
					if hasError != nil {
						if errs.IsSoleCancelCause(hasError) {
							log.Warn().Msg("Hashing canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to calculate hash")
							widgets.ShowError(t.Window, "Hashing Error", fmt.Sprintf("Failed to calculate hash: %v", hasError))
						}
					} else {
						log.Info().
							Str("file", filePath).
							Int("algorithms", len(t.resolveSelectedAlgorithms())).
							Msg("Hashing completed")
					}

					t.CancelOperation()
					t.setStartState()
				})
			},
		})
	}()
}

func (t *HashTab) onStop() {
	t.CancelOperation()
}

func (t *HashTab) activateStopState() {
	t.clearHashResults()
	t.updateStats(0)

	t.btnStart.SetVisible(false)
	t.btnStop.SetVisible(true)
	t.frameHashProgress.SetVisible(true)
	t.btnBrowseFile.SetSensitive(false)
	t.entryFile.SetSensitive(false)
	t.chkHashOnOpen.SetSensitive(false)
	t.treeHash.SetSensitive(false)

	t.applySearchHighlighting()
}

func (t *HashTab) setStartState() {
	t.btnStart.SetVisible(true)
	t.btnStop.SetVisible(false)
	t.frameHashProgress.SetVisible(false)
	t.btnBrowseFile.SetSensitive(true)
	t.entryFile.SetSensitive(true)
	t.chkHashOnOpen.SetSensitive(true)
	t.treeHash.SetSensitive(true)

	t.applySearchHighlighting()
}

func (t *HashTab) updateStats(progress float64) {
	t.progressBar.SetFraction(progress)
}

func (t *HashTab) updateHashForAlgorithm(algo algorithm.Algorithm, hash string) {
	if hash == "" {
		return
	}

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		ext, ok := widgets.ListStoreString(t.listStore, iter, 2)
		if !ok {
			return true
		}

		if ext == algo.Extension() {
			if err := t.listStore.SetValue(iter, 1, hash); err != nil {
				widgets.MustWidget("ListStore", "HashTab.updateSearchHighlight", err)
			}

			return false
		}

		return true
	})
}

func (t *HashTab) clearHashResults() {
	t.forEachRow(func(iter *gtk.TreeIter) bool {
		if err := t.listStore.SetValue(iter, 1, ""); err != nil {
			widgets.MustWidget("ListStore", "HashTab.clearHashResults", err)
		}

		return true
	})
}

func (t *HashTab) applySettingsToUI() {
	t.populateAlgorithmTable()
	t.chkHashOnOpen.SetActive(t.Settings.Hash.HashOnOpen)
}

func (t *HashTab) saveSettings() error {
	if !widgets.IsAlive(t.Window) {
		return nil
	}

	t.Settings.Hash.Algorithms = t.getSelectedAlgorithms()
	t.Settings.Hash.HashOnOpen = t.chkHashOnOpen.GetActive()

	return t.Settings.Save()
}

func (t *HashTab) setupSearchCSS() {
	cssProvider, err := gtk.CssProviderNew()
	if err != nil {
		widgets.MustWidget("CssProvider", "HashTab.setupSearchCSS", err)
	}

	css := `
		.search-found {
			background-color: green;
		}
		.search-not-found {
			background-color: firebrick1;
		}
	`
	if err := cssProvider.LoadFromData(css); err != nil {
		widgets.MustWidget("CssProvider.LoadFromData", "HashTab.setupSearchCSS", err)
	}

	screen, err := t.searchEntry.GetScreen()
	if err != nil {
		widgets.MustWidget("Screen", "HashTab.setupSearchCSS", err)
	}

	gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

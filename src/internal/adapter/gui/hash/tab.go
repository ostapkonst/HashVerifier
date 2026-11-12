// Package hash implements the Hash GUI tab: single-file hash with per-algorithm toggles, export, and search.
package hash

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/appmeta"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
	"github.com/ostapkonst/HashVerifier/internal/service/hash"
)

// HashTab embeds base.TabBase and adds GTK widgets and callbacks for the Hash use-case
// (single file, per-algorithm toggles).
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
	toggleColumn        *gtk.TreeViewColumn
	hdrChkAllAlgos      *gtk.CheckButton
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
	tab.setupHeaderCheckbox()
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
	t.toggleColumn = widgets.GetTreeViewColumn(t.Builder, "tree_hash_column_toggle")
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

	t.updateHeaderCheckboxState()
}

func (t *HashTab) setupHandlers() {
	t.btnBrowseFile.Connect("clicked", func() {
		path, _ := t.entryFile.GetText()
		if file, ok := widgets.OpenAnyFileDialog(t.Window, "Select File to Hash", path); ok {
			t.entryFile.SetText(file)
			t.enableEmbeddedChecksumAlgorithm(file)

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

	styleContext, err := t.searchEntry.GetStyleContext()
	if err != nil {
		widgets.MustWidget("StyleContext", "HashTab.applySearchHighlighting:GetStyleContext", err)
	}

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
		"Export hash",
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
			fmt.Sprintf("Source file is unavailable:\n%s", err))

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

	// The output must not be the file being hashed (or a hard-link/symlink alias): writing the
	// checksum line over the source would destroy its content.
	if same, err := fs.SameFile(sourcePath, savePath); err != nil {
		widgets.ShowError(t.Window, "Export Hash",
			fmt.Sprintf("Cannot compare source and output:\n%s", err))

		return
	} else if same {
		widgets.ShowError(t.Window, "Export Hash",
			fmt.Sprintf("Refusing to overwrite the file being hashed:\n%s", savePath))

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

	// The hash of any file is computable; only writing it into a checksum line a parser
	// cannot recover (SFV ';' comment / eaten trailing space) is refused, here at export time.
	if perr := hashfn.ValidatePathForFormat(relPath, algoType); perr != nil {
		widgets.ShowError(t.Window, "Export Hash",
			fmt.Sprintf("Cannot export %s as checksum line:\n%s", relPath, perr))

		return
	}

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

func (t *HashTab) setupHeaderCheckbox() {
	hdrChk, err := gtk.CheckButtonNew()
	if err != nil {
		widgets.MustWidget("CheckButton", "HashTab.setupHeaderCheckbox", err)
	}

	hdrChk.SetCanFocus(false)
	hdrChk.SetHAlign(gtk.ALIGN_CENTER)
	hdrChk.SetVAlign(gtk.ALIGN_CENTER)

	t.toggleColumn.SetWidget(hdrChk)
	t.hdrChkAllAlgos = hdrChk

	t.hdrChkAllAlgos.ShowAll()

	t.toggleColumn.Connect("clicked", t.onHeaderClicked)
}

func (t *HashTab) onHeaderClicked() {
	shouldCheckAll := !t.hdrChkAllAlgos.GetActive()

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		if err := t.listStore.SetValue(iter, 3, shouldCheckAll); err != nil {
			widgets.MustWidget("ListStore", "HashTab.onHeaderClicked", err)
		}

		return true
	})

	t.hdrChkAllAlgos.SetInconsistent(false)
	t.hdrChkAllAlgos.SetActive(shouldCheckAll)

	if err := t.saveSettings(); err != nil {
		t.LogError("save hash settings", err)
	}
}

func (t *HashTab) updateHeaderCheckboxState() {
	enabled, total := 0, 0

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		total++

		if v, ok := widgets.ListStoreBool(t.listStore, iter, 3); ok && v {
			enabled++
		}

		return true
	})

	t.hdrChkAllAlgos.SetInconsistent(enabled > 0 && enabled < total)
	t.hdrChkAllAlgos.SetActive(enabled == total)
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

	t.updateHeaderCheckboxState()
}

// setAlgorithmEnabled checks or unchecks the row of the given algorithm and refreshes the
// header checkbox state.
func (t *HashTab) setAlgorithmEnabled(algo algorithm.Algorithm, enabled bool) {
	ext := algo.Extension()

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		rowExt, ok := widgets.ListStoreString(t.listStore, iter, 2)
		if !ok || rowExt != ext {
			return true
		}

		if err := t.listStore.SetValue(iter, 3, enabled); err != nil {
			widgets.MustWidget("ListStore", "HashTab.setAlgorithmEnabled", err)
		}

		return false
	})

	t.updateHeaderCheckboxState()
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
	t.enableEmbeddedChecksumAlgorithm(path)

	if t.chkHashOnOpen.GetActive() {
		t.onStart()
	}

	return nil
}

// enableEmbeddedChecksumAlgorithm checks the CRC-32 row and fills the desired-hash field when
// the file name carries an embedded CRC-32, so the computed checksum can be compared with the
// one the name claims. The choice is saved, keeping table, setting and file in agreement.
func (t *HashTab) enableEmbeddedChecksumAlgorithm(path string) {
	checksum, ok := embedded.Checksum(filepath.Base(path))
	if !ok {
		return
	}

	t.setAlgorithmEnabled(algorithm.CRC32, true)
	t.searchEntry.SetText(checksum)

	if err := t.saveSettings(); err != nil {
		t.LogError("save hash settings", err)
	}
}

// FocusMainAction moves keyboard focus to the primary action button.
func (t *HashTab) FocusMainAction() {
	t.btnStart.GrabFocus()
}

func (t *HashTab) validateInputs(filePath string) bool {
	if filePath == "" {
		widgets.ShowError(t.Window, "No File Selected", "Please select a file to hash.")

		return false
	}

	// Validate the cleaned path, the same one the run uses: a trailing separator must
	// not make an existing file report as unavailable.
	filePath = filepath.Clean(filePath)

	if err := hash.ValidateFilePath(filePath); err != nil {
		widgets.ShowError(t.Window, "Invalid Path",
			fmt.Sprintf("Source file is unavailable:\n%s", err))

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

	// Wg.Add(1) immediately after the synchronous validation so a shutdown during the async
	// setup below still waits for the goroutine and never leaks a started RunStream.
	t.Wg.Add(1)

	crash.Go("hashTab.onStart", func() {
		defer t.Wg.Done()
		defer t.CancelOperation()

		ctx, cancel := context.WithCancel(t.Ctx)
		t.SetCancel(cancel)

		cfg := hash.HashConfig{
			FilePath:   filePath,
			Algorithms: selectedAlgos,
		}

		results, err := hash.HashFileStreaming(ctx, cfg)
		if err != nil {
			widgets.IdleAdd(t.Window, func() {
				t.setStartState()
				widgets.ShowError(t.Window, "Hashing Error", fmt.Sprintf("Failed to start hashing: %v", err))
			})

			return
		}

		log.Info().
			Str("file", filePath).
			Strs("algorithms", algoNames(selectedAlgos)).
			Msg("Starting hashing")

		widgets.RunStream(results, widgets.StreamBatchConfig[hash.HashStreamingResult]{
			FlushSize:     widgets.StreamFlushSize,
			FlushInterval: widgets.StreamFlushInterval,
			IsProgress:    func(r hash.HashStreamingResult) bool { return r.IsProgressUpdate },
			GetError:      func(r hash.HashStreamingResult) error { return r.Err },
			OnProgress: func(r hash.HashStreamingResult) {
				progress := r.Progress

				widgets.IdleAddPriority(t.Window, glib.PRIORITY_HIGH_IDLE, func() {
					t.updateStats(progress)
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
						if errs.IsContextDone(hasError) {
							log.Warn().Msg("Hashing canceled")
						} else {
							log.Error().Err(hasError).Msg("Failed to calculate hash")
							widgets.ShowError(t.Window, "Hashing Error", fmt.Sprintf("Failed to calculate hash: %v", hasError))
						}
					} else {
						log.Info().
							Str("file", filePath).
							Int("algorithms", len(selectedAlgos)).
							Msg("Hashing completed")
					}

					t.setStartState()
				})
			},
		})
	})
}

func (t *HashTab) onStop() {
	t.CancelOperation()
}

func (t *HashTab) activateStopState() {
	t.clearHashResults()
	t.updateStats(0)
	widgets.ResetColumnWidths(t.treeHash)

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
				widgets.MustWidget("ListStore", "HashTab.updateHashForAlgorithm", err)
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

	return t.Settings.AsyncSave()
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

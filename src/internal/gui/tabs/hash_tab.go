package tabs

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/action/hasher"
	"github.com/ostapkonst/HashVerifier/internal/gui/widgets"
	"github.com/ostapkonst/HashVerifier/internal/settings"
)

type HashTab struct {
	*TabBase
	entryFile           *gtk.Entry
	btnStart            *gtk.Button
	btnStop             *gtk.Button
	btnBrowseFile       *gtk.Button
	treeHash            *gtk.TreeView
	listStore           *gtk.ListStore
	chkHashOnOpen       *gtk.CheckButton
	gridHashProgress    *gtk.Frame
	progressBar         *gtk.ProgressBar
	cellRendererToggle  *gtk.CellRendererToggle
	contextMenuProvider *widgets.ContextMenuProvider
	searchEntry         *gtk.SearchEntry
}

func NewHashTab(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings) *HashTab {
	tab := &HashTab{
		TabBase: NewTabBase(ctx, builder, window, settings, nil),
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
	t.gridHashProgress = widgets.GetFrame(t.Builder, "grid_hash_progress")
	t.progressBar = widgets.GetProgressBar(t.Builder, "progress_hash")
	t.cellRendererToggle = widgets.GetCellRendererToggle(t.Builder, "cell_renderer_hash_toggle")
	t.searchEntry = widgets.GetSearchEntry(t.Builder, "search_hash_in_table")
}

func (t *HashTab) populateAlgorithmTable() {
	t.listStore.Clear()

	algorithms := []struct {
		ext  string
		name string
	}{
		{".sfv", "CRC-32"},
		{".md4", "MD4"},
		{".md5", "MD5"},
		{".sha1", "SHA-1"},
		{".sha256", "SHA-256"},
		{".sha384", "SHA-384"},
		{".sha512", "SHA-512"},
		{".sha3-256", "SHA3-256"},
		{".sha3-384", "SHA3-384"},
		{".sha3-512", "SHA3-512"},
		{".blake3", "BLAKE3"},
		{".xxh3", "XXH3"},
		{".xxh128", "XXH128"},
	}

	enabledAlgos := make(map[string]bool)
	for _, algoExt := range t.Settings.Hash.Algorithms {
		enabledAlgos[algoExt] = true
	}

	for _, algoInfo := range algorithms {
		iter := t.listStore.Append()
		_ = t.listStore.SetValue(iter, 0, algoInfo.name)
		_ = t.listStore.SetValue(iter, 1, "")
		_ = t.listStore.SetValue(iter, 2, algoInfo.ext)
		_ = t.listStore.SetValue(iter, 3, enabledAlgos[algoInfo.ext])
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
		hashVal, err := t.listStore.GetValue(iter, 1) // hashsum
		if err == nil {
			hashGo, _ := hashVal.GoValue()
			if hash, _ := hashGo.(string); hash != "" {
				hashes = append(hashes, hash)
			}
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
	t.contextMenuProvider.CreateSimpleMenu([]int{0, 1}, columnLabels)
	t.contextMenuProvider.ConnectRightClick(func() {
		t.contextMenuProvider.ShowMenu()
	})
}

func (t *HashTab) setupToggleHandler() {
	if t.cellRendererToggle == nil {
		return
	}

	t.cellRendererToggle.Connect("toggled", func(renderer *gtk.CellRendererToggle, pathStr string) {
		path, err := gtk.TreePathNewFromString(pathStr)
		if err != nil {
			return
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
		return
	}

	val, err := t.listStore.GetValue(iter, 3) // calc_enabled
	if err != nil {
		return
	}

	goVal, err := val.GoValue()
	if err != nil {
		return
	}

	currentState := goVal.(bool)
	_ = t.listStore.SetValue(iter, 3, !currentState)
}

func (t *HashTab) getSelectedAlgorithms() []string {
	var selected []string

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		enabledVal, err := t.listStore.GetValue(iter, 3) // calc_enabled
		if err != nil {
			return true
		}

		goVal, err := enabledVal.GoValue()
		if err != nil {
			return true
		}

		if !goVal.(bool) {
			return true
		}

		extVal, err := t.listStore.GetValue(iter, 2) // extension
		if err != nil {
			return true
		}

		extGo, _ := extVal.GoValue()
		selected = append(selected, extGo.(string))

		return true
	})

	return selected
}

func (t *HashTab) Fill(path string) error {
	if t.IsBusy() {
		return ErrTabBusy
	}

	t.entryFile.SetText(path)

	if t.chkHashOnOpen.GetActive() {
		t.onStart()
	}

	return nil
}

func (t *HashTab) onStart() {
	filePath, _ := t.entryFile.GetText()
	filePath = filepath.Clean(filePath)
	selectedAlgos := t.getSelectedAlgorithms()

	t.activateStopState()

	ctx, cancel := context.WithCancel(t.Ctx)
	t.Cancel = cancel

	cfg := hasher.HashConfig{
		FilePath:   filePath,
		Algorithms: selectedAlgos,
	}

	results, err := hasher.HashFileStreaming(ctx, cfg)
	if err != nil {
		t.CancelOperation()
		t.setStartState()

		widgets.ShowError(t.Window, "Hashing Error", fmt.Sprintf("Failed to start hashing: %v", err))

		return
	}

	log.Info().
		Str("file", filePath).
		Strs("algorithms", selectedAlgos).
		Msg("Starting hash calculation")

	t.Wg.Add(1)

	var hasError error

	go func() {
		defer t.Wg.Done()

		for res := range results {
			if res.IsProgressUpdate {
				glib.IdleAdd(func() {
					t.updateStats(res.Progress)
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
				t.updateHashResult(res)
				t.updateStats(res.Progress)
			})
		}
	}()

	go func() {
		t.Wg.Wait()
		func() {
			if hasError != nil {
				if errors.Is(hasError, context.Canceled) {
					log.Warn().Msg("Hash calculation canceled")
					return
				}

				log.Error().Err(hasError).Msg("Failed to calculate hash")
				glib.IdleAdd(func() {
					widgets.ShowError(t.Window, "Hashing Error", fmt.Sprintf("Failed to calculate hash: %v", hasError))
				})

				return
			}

			log.Info().Msg("Hash calculation completed")
		}()
		glib.IdleAdd(func() {
			t.CancelOperation()
			t.setStartState()
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
	t.gridHashProgress.SetVisible(true)
	t.btnBrowseFile.SetSensitive(false)
	t.entryFile.SetSensitive(false)
	t.chkHashOnOpen.SetSensitive(false)
	t.treeHash.SetSensitive(false)

	t.applySearchHighlighting()
}

func (t *HashTab) setStartState() {
	t.btnStart.SetVisible(true)
	t.btnStop.SetVisible(false)
	t.gridHashProgress.SetVisible(false)
	t.btnBrowseFile.SetSensitive(true)
	t.entryFile.SetSensitive(true)
	t.chkHashOnOpen.SetSensitive(true)
	t.treeHash.SetSensitive(true)

	t.applySearchHighlighting()
}

func (t *HashTab) updateStats(progress float64) {
	t.progressBar.SetFraction(progress)
}

func (t *HashTab) updateHashResult(res hasher.HashStreamingResult) {
	if res.Result.Hash == "" {
		return
	}

	t.forEachRow(func(iter *gtk.TreeIter) bool {
		extVal, err := t.listStore.GetValue(iter, 2) // extension
		if err != nil {
			return true
		}

		extGo, _ := extVal.GoValue()
		if extGo.(string) == res.Result.Algorithm.Extension() {
			_ = t.listStore.SetValue(iter, 1, res.Result.Hash) // hashsum
			return false                                       // нашли, останавливаемся
		}

		return true
	})
}

func (t *HashTab) clearHashResults() {
	t.forEachRow(func(iter *gtk.TreeIter) bool {
		_ = t.listStore.SetValue(iter, 1, "") // hashsum
		return true
	})
}

func (t *HashTab) applySettingsToUI() {
	t.populateAlgorithmTable()
	t.chkHashOnOpen.SetActive(t.Settings.Hash.HashOnOpen)
}

func (t *HashTab) saveSettings() error {
	if t.Window.InDestruction() {
		return nil
	}

	t.Settings.Hash.Algorithms = t.getSelectedAlgorithms()
	t.Settings.Hash.HashOnOpen = t.chkHashOnOpen.GetActive()

	return t.Settings.Save()
}

func (t *HashTab) setupSearchCSS() {
	cssProvider, _ := gtk.CssProviderNew()
	css := `
		.search-found {
			background-color: green;
		}
		.search-not-found {
			background-color: firebrick1;
		}
	`
	_ = cssProvider.LoadFromData(css)

	screen, err := t.searchEntry.GetScreen()
	if err != nil {
		return
	}

	gtk.AddProviderForScreen(screen, cssProvider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

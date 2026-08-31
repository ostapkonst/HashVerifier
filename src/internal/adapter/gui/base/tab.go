// Package base provides shared infrastructure for every GUI tab: TabBase, ProgressTracker, and ErrTabBusy.
package base

import (
	"context"
	"fmt"
	"sync"

	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/adapter/gui/widgets"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
)

// TabBase carries the per-tab runtime state (context, cancellation, wait-group, settings, builder, window). All three tabs embed it.
type TabBase struct {
	Ctx          context.Context
	Cancel       context.CancelFunc
	cancelMu     sync.Mutex // guards Cancel: assigned on the GTK thread, cleared from shutdown callbacks
	Wg           sync.WaitGroup
	Settings     *settings.Settings
	ColumnConfig *widgets.ColumnConfig
	Builder      *gtk.Builder
	Window       *gtk.Window
}

// NewTabBase wires shared dependencies into a TabBase; Cancel is assigned by the tab's onStart handler after construction.
func NewTabBase(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings, columnConfig *widgets.ColumnConfig) *TabBase {
	return &TabBase{
		Ctx:          ctx,
		Builder:      builder,
		Window:       window,
		Settings:     settings,
		ColumnConfig: columnConfig,
	}
}

// Wait blocks until every background worker tracked by this tab has returned.
func (tb *TabBase) Wait() {
	tb.Wg.Wait()
}

// CancelOperation cancels the running operation and clears the cancel func so IsBusy flips to false afterwards.
// Safe to call from any goroutine: the cancel func is read and cleared under cancelMu.
func (tb *TabBase) CancelOperation() {
	tb.cancelMu.Lock()
	cancel := tb.Cancel
	tb.Cancel = nil
	tb.cancelMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// SetCancel stores the cancel func for the run started by the tab's onStart handler.
func (tb *TabBase) SetCancel(cancel context.CancelFunc) {
	tb.cancelMu.Lock()
	defer tb.cancelMu.Unlock()

	tb.Cancel = cancel
}

// SetupColumnHandlers wires columns-changed and per-column clicked to onColumnChanged for reorders or header clicks.
func (tb *TabBase) SetupColumnHandlers(treeView *gtk.TreeView, onColumnChanged func()) {
	treeView.Connect("columns-changed", onColumnChanged)

	columns := treeView.GetColumns()
	for l := columns; l != nil; l = l.Next() {
		if col, ok := l.Data().(*gtk.TreeViewColumn); ok {
			col.Connect("clicked", onColumnChanged)
		}
	}
}

// ApplySortOrder converts a settings.SortOrder to GTK and applies it plus the sort column to the given treeView.
func (tb *TabBase) ApplySortOrder(treeView *gtk.TreeView, sortColumn string, sortOrder settings.SortOrder) {
	var gtkSortOrder gtk.SortType
	if sortOrder == settings.SortOrderDesc {
		gtkSortOrder = gtk.SORT_DESCENDING
	} else {
		gtkSortOrder = gtk.SORT_ASCENDING
	}

	tb.ColumnConfig.ApplySortState(treeView, sortColumn, gtkSortOrder)
}

// LogError writes an error-level zerolog entry tagged with the tab-level operation name.
func (tb *TabBase) LogError(operation string, err error) {
	log.Error().Err(err).Str("operation", operation).Msg("Failed to save settings")
}

// IsBusy reports whether this tab currently has a live operation that could be canceled.
func (tb *TabBase) IsBusy() bool {
	tb.cancelMu.Lock()
	defer tb.cancelMu.Unlock()

	return tb.Cancel != nil
}

// WindowAlive reports whether the main window is not in destruction; prefer widgets.IdleAdd
// for posting closures to the GTK main loop, which applies this guard automatically.
// Use WindowAlive directly only when widgets.IdleAdd is not an option (e.g. an existing
// raw glib.IdleAdd callback that cannot easily be retargeted).
func (tb *TabBase) WindowAlive() bool {
	return !tb.Window.InDestruction()
}

// SetStatLabel writes a "value of total files" caption and applies color only once work has begun.
func SetStatLabel(label *gtk.Label, value, total int, color string) {
	text := fmt.Sprintf("%d of %d files", value, total)
	if value > 0 && color != "" {
		label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
	} else {
		label.SetText(text)
	}
}

// SetFinalLabel writes the post-run "value of total files" caption in the supplied color.
func SetFinalLabel(label *gtk.Label, value, total int, color string) {
	text := fmt.Sprintf("%d of %d files", value, total)
	label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
}

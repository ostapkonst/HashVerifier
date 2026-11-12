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

// TabBase carries the per-tab runtime state (context, cancellation, wait-group, settings,
// builder, window). All tabs embed it.
type TabBase struct {
	Ctx          context.Context
	Cancel       context.CancelFunc
	cancelMu     sync.Mutex // guards Cancel: set by the worker goroutine, cleared by CancelOperation
	Wg           sync.WaitGroup
	Settings     *settings.Settings
	ColumnConfig *widgets.ColumnConfig
	Builder      *gtk.Builder
	Window       *gtk.Window
	StartButton  *gtk.Button
	StopButton   *gtk.Button
	sortRestore  func()
	runGate      *RunGate
	tabName      string
}

// NewTabBase wires shared dependencies into a TabBase; Cancel is assigned by the tab's
// onStart handler after construction.
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

// FocusMainAction moves keyboard focus to the tab's primary action button.
func (tb *TabBase) FocusMainAction() {
	tb.StartButton.GrabFocus()
}

// SetStartEnabled enables or disables the primary action button while another tab runs.
func (tb *TabBase) SetStartEnabled(enabled bool) {
	tb.StartButton.SetSensitive(enabled)
}

// SetRunGate wires the shared run gate and this tab's notebook page name.
func (tb *TabBase) SetRunGate(gate *RunGate, name string) {
	tb.runGate = gate
	tb.tabName = name
}

// LockRun marks this tab as the active operation so the app disables the other tabs' action buttons.
func (tb *TabBase) LockRun() {
	if tb.runGate != nil {
		tb.runGate.Lock(tb.tabName)
	}
}

// UnlockRun releases the active-operation lock when this tab owns it.
func (tb *TabBase) UnlockRun() {
	if tb.runGate != nil {
		tb.runGate.Unlock(tb.tabName)
	}
}

// RunActive reports whether any tab currently holds the run gate.
func (tb *TabBase) RunActive() bool {
	return tb.runGate != nil && tb.runGate.IsLocked()
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

// SuspendSort captures the TreeView sort state and turns sorting off for a bulk row run so the
// store is not resorted per insertion. Must be called on the GTK thread; re-entry is a no-op.
func (tb *TabBase) SuspendSort(treeView *gtk.TreeView) {
	if tb.sortRestore != nil {
		return
	}

	tb.sortRestore = tb.ColumnConfig.SuspendSort(treeView)
}

// RestoreSort reinstates the state captured by SuspendSort; a no-op when nothing is suspended.
// Must be called on the GTK thread.
func (tb *TabBase) RestoreSort() {
	if tb.sortRestore == nil {
		return
	}

	tb.sortRestore()
	tb.sortRestore = nil
}

// SortSuspended reports whether sorting is currently suspended for a bulk run.
func (tb *TabBase) SortSuspended() bool {
	return tb.sortRestore != nil
}

// LogError writes an error-level zerolog entry with operation in the structured field; the hard-coded
// Msg "Failed to save settings" reflects this helper's only call sites (save failures in the tabs).
func (tb *TabBase) LogError(operation string, err error) {
	log.Error().Err(err).Str("operation", operation).Msg("Failed to save settings")
}

// IsBusy reports whether this tab currently has a live operation that could be canceled.
func (tb *TabBase) IsBusy() bool {
	tb.cancelMu.Lock()
	defer tb.cancelMu.Unlock()

	return tb.Cancel != nil
}

// SetStatLabel writes an "N files" caption and applies color only once work has begun.
func SetStatLabel(label *gtk.Label, value int, color string) {
	text := fmt.Sprintf("%d files", value)
	if value > 0 && color != "" {
		label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
	} else {
		label.SetText(text)
	}
}

// SetFinalLabel writes the post-run "N files" caption in the supplied color.
func SetFinalLabel(label *gtk.Label, value int, color string) {
	text := fmt.Sprintf("%d files", value)
	label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
}

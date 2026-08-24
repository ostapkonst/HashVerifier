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
	Wg           sync.WaitGroup
	Settings     *settings.Settings
	ColumnConfig *widgets.ColumnConfig
	Builder      *gtk.Builder
	Window       *gtk.Window
}

// NewTabBase constructs a TabBase embedding the shared runtime state.
func NewTabBase(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings, columnConfig *widgets.ColumnConfig) *TabBase {
	return &TabBase{
		Ctx:          ctx,
		Builder:      builder,
		Window:       window,
		Settings:     settings,
		ColumnConfig: columnConfig,
	}
}

func (tb *TabBase) Wait() {
	tb.Wg.Wait()
}

func (tb *TabBase) CancelOperation() {
	if tb.Cancel != nil {
		tb.Cancel()
	}

	tb.Cancel = nil
}

// SetupColumnHandlers wires the columns-changed and per-column clicked signals so onColumnChanged runs whenever the user reorders or clicks a column header.
func (tb *TabBase) SetupColumnHandlers(treeView *gtk.TreeView, onColumnChanged func()) {
	treeView.Connect("columns-changed", onColumnChanged)

	columns := treeView.GetColumns()
	for l := columns; l != nil; l = l.Next() {
		if col, ok := l.Data().(*gtk.TreeViewColumn); ok {
			col.Connect("clicked", onColumnChanged)
		}
	}
}

func (tb *TabBase) ApplySortOrder(treeView *gtk.TreeView, sortColumn string, sortOrder settings.SortOrder) {
	var gtkSortOrder gtk.SortType
	if sortOrder == settings.SortOrderDesc {
		gtkSortOrder = gtk.SORT_DESCENDING
	} else {
		gtkSortOrder = gtk.SORT_ASCENDING
	}

	tb.ColumnConfig.ApplySortState(treeView, sortColumn, gtkSortOrder)
}

func (tb *TabBase) LogError(operation string, err error) {
	log.Error().Err(err).Str("operation", operation).Msg("Failed to save settings")
}

func (tb *TabBase) IsBusy() bool {
	return tb.Cancel != nil
}

func SetStatLabel(label *gtk.Label, value, total int, color string) {
	text := fmt.Sprintf("%d of %d files", value, total)
	if value > 0 && color != "" {
		label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
	} else {
		label.SetText(text)
	}
}

func SetFinalLabel(label *gtk.Label, value, total, pending int, color string) {
	text := fmt.Sprintf("%d of %d files", value, total)
	label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
}

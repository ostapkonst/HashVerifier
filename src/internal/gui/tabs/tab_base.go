package tabs

import (
	"context"
	"fmt"
	"sync"

	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/settings"
)

type TabBase struct {
	Ctx          context.Context
	Cancel       context.CancelFunc
	Wg           sync.WaitGroup
	Settings     *settings.Settings
	ColumnConfig *ColumnConfig
	Builder      *gtk.Builder
	Window       *gtk.Window
}

func NewTabBase(ctx context.Context, builder *gtk.Builder, window *gtk.Window, settings *settings.Settings, columnConfig *ColumnConfig) *TabBase {
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

func setStatLabel(label *gtk.Label, value, total int, color string) {
	text := fmt.Sprintf("%d of %d files", value, total)
	if value > 0 && color != "" {
		label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
	} else {
		label.SetText(text)
	}
}

func setFinalLabel(label *gtk.Label, value, total, errorCount, pending int, color string) {
	text := fmt.Sprintf("%d of %d files", value, total)
	if value > 0 && pending == 0 {
		label.SetMarkup(fmt.Sprintf(`<span foreground="%s">%s</span>`, color, text))
	} else {
		label.SetText(text)
	}
}

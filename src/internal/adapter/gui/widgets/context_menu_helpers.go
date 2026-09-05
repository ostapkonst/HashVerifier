package widgets

import (
	"fmt"
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"
)

// ContextMenuProvider builds a right-click context menu over a TreeView, optionally with a "Reveal in file manager" or "Export" row.
type ContextMenuProvider struct {
	treeView  *gtk.TreeView
	listStore *gtk.ListStore
	menu      *gtk.Menu
}

// NewContextMenuProvider binds a context-menu helper to the given TreeView and its underlying ListStore.
func NewContextMenuProvider(treeView *gtk.TreeView, listStore *gtk.ListStore) *ContextMenuProvider {
	return &ContextMenuProvider{
		treeView:  treeView,
		listStore: listStore,
	}
}

// ConnectRightClick selects the row under the cursor on right-click and invokes onShowMenu before the menu pops.
func (p *ContextMenuProvider) ConnectRightClick(onShowMenu func()) {
	p.treeView.Connect("button-press-event", func(_ *gtk.TreeView, event *gdk.Event) bool {
		eventButton := gdk.EventButtonNewFromEvent(event)
		if eventButton.Button() != 3 {
			return false
		}

		path, _, _, _, ok := p.treeView.GetPathAtPos(int(eventButton.X()), int(eventButton.Y()))
		if !ok {
			return false
		}

		selection, err := p.treeView.GetSelection()
		if err != nil {
			MustWidget("TreeView", "ContextMenuProvider.ConnectRightClick:GetSelection", err)
		}

		selection.SelectPath(path)

		if onShowMenu != nil {
			onShowMenu()
		}

		return true
	})
}

// CreateMenuWithReveal builds a menu prepended with "Show in Explorer" plus copy entries ("Copy full path", "Copy dir path", per-column copies), storing it for ShowMenu.
func (p *ContextMenuProvider) CreateMenuWithReveal(
	fullPathIdx int,
	columnLabels []string,
	onReveal func(fullPath string),
) {
	revealItem, err := gtk.MenuItemNewWithLabel("Show in Explorer")
	if err != nil {
		MustWidget("MenuItem", "ContextMenuProvider.CreateMenuWithReveal", err)
	}

	revealItem.Connect("activate", func() {
		rowData, ok := getSelectedRowData(p.treeView, p.listStore)
		if !ok {
			return
		}

		fullPath, exists := rowData[fullPathIdx]
		if !exists || fullPath == "" {
			return
		}

		if onReveal != nil {
			onReveal(fullPath)
		}
	})

	separator, err := gtk.SeparatorMenuItemNew()
	if err != nil {
		MustWidget("SeparatorMenuItem", "ContextMenuProvider.CreateMenuWithReveal", err)
	}

	prepend := []gtk.IMenuItem{revealItem, separator}
	p.menu = p.buildCopySubmenu(fullPathIdx, columnLabels, prepend)
	p.menu.ShowAll()
}

// CreateMenuWithExportItem builds a menu with a custom export item followed by per-column copy entries, stored for ShowMenu.
func (p *ContextMenuProvider) CreateMenuWithExportItem(exportLabel string, onExport func(), columnIndices []int, columnLabels []string) {
	menu, err := gtk.MenuNew()
	if err != nil {
		MustWidget("Menu", "ContextMenuProvider.CreateMenuWithExportItem", err)
	}

	exportItem, err := gtk.MenuItemNewWithLabel(exportLabel)
	if err != nil {
		MustWidget("MenuItem", "ContextMenuProvider.CreateMenuWithExportItem", err)
	}

	exportItem.Connect("activate", func() {
		if onExport != nil {
			onExport()
		}
	})
	menu.Append(exportItem)

	separator, err := gtk.SeparatorMenuItemNew()
	if err != nil {
		MustWidget("SeparatorMenuItem", "ContextMenuProvider.CreateMenuWithExportItem", err)
	}

	menu.Append(separator)

	for i, idx := range columnIndices {
		label := columnLabels[i]

		copyItem, err := gtk.MenuItemNewWithLabel(fmt.Sprintf("Copy %s", label))
		if err != nil {
			MustWidget("MenuItem", "ContextMenuProvider.CreateMenuWithExportItem", err)
		}

		copyItem.Connect("activate", func() {
			p.copyColumnValue(idx, nil)
		})
		menu.Append(copyItem)
	}

	menu.ShowAll()
	p.menu = menu
}

func (p *ContextMenuProvider) buildCopySubmenu(fullPathIdx int, columnLabels []string, prepend []gtk.IMenuItem) *gtk.Menu {
	menu, err := gtk.MenuNew()
	if err != nil {
		MustWidget("Menu", "ContextMenuProvider.buildCopySubmenu", err)
	}

	for _, item := range prepend {
		menu.Append(item)
	}

	copyItem, err := gtk.MenuItemNewWithLabel("Copy full path")
	if err != nil {
		MustWidget("MenuItem", "ContextMenuProvider.buildCopySubmenu", err)
	}

	copyItem.Connect("activate", func() {
		p.copyColumnValue(fullPathIdx, nil)
	})
	menu.Append(copyItem)

	copyItem, err = gtk.MenuItemNewWithLabel("Copy dir path")
	if err != nil {
		MustWidget("MenuItem", "ContextMenuProvider.buildCopySubmenu", err)
	}

	copyItem.Connect("activate", func() {
		p.copyColumnValue(fullPathIdx, filepath.Dir)
	})
	menu.Append(copyItem)

	if len(columnLabels) > 0 {
		separator, err := gtk.SeparatorMenuItemNew()
		if err != nil {
			MustWidget("SeparatorMenuItem", "ContextMenuProvider.buildCopySubmenu", err)
		}

		menu.Append(separator)
	}

	for i, label := range columnLabels {
		copyItem, err := gtk.MenuItemNewWithLabel(fmt.Sprintf("Copy %s", label))
		if err != nil {
			MustWidget("MenuItem", "ContextMenuProvider.buildCopySubmenu", err)
		}

		copyItem.Connect("activate", func() {
			p.copyColumnValue(i, nil)
		})
		menu.Append(copyItem)
	}

	return menu
}

// ShowMenu pops the previously built context menu at the pointer; no-op when no menu has been created yet.
func (p *ContextMenuProvider) ShowMenu() {
	if p.menu == nil {
		return
	}

	p.menu.PopupAtPointer(nil)
}

func (p *ContextMenuProvider) copyColumnValue(colIndex int, processingFn func(string) string) {
	rowData, ok := getSelectedRowData(p.treeView, p.listStore)
	if !ok {
		return
	}

	if value, exists := rowData[colIndex]; exists {
		if processingFn != nil {
			value = processingFn(value)
		}

		if err := copyToClipboard(value); err != nil {
			log.Warn().Err(err).Str("operation", "ContextMenuProvider.copyColumnValue").Msg("Failed to copy to clipboard")
		}
	}
}

func copyToClipboard(text string) error {
	clipboard, err := gtk.ClipboardGet(gdk.SELECTION_CLIPBOARD)
	if err != nil {
		return fmt.Errorf("failed to get clipboard: %w", err)
	}

	clipboard.SetText(text)
	clipboard.Store()

	return nil
}

func getSelectedRowData(treeView *gtk.TreeView, listStore *gtk.ListStore) (map[int]string, bool) {
	selection, err := treeView.GetSelection()
	if err != nil {
		MustWidget("TreeView", "getSelectedRowData:GetSelection", err)
	}

	_, iter, ok := selection.GetSelected()
	if !ok {
		return nil, false
	}

	columns := listStore.GetNColumns()

	rowData := make(map[int]string, columns)
	for i := range columns {
		value, err := listStore.GetValue(iter, i)
		if err != nil {
			continue
		}

		goVal, err := value.GoValue()
		if err != nil {
			continue
		}

		switch v := goVal.(type) {
		case string:
			rowData[i] = v
		case int, int64, uint, uint64:
			rowData[i] = fmt.Sprintf("%d", v)
		case float64:
			rowData[i] = fmt.Sprintf("%g", v)
		default:
			rowData[i] = fmt.Sprintf("%v", v)
		}
	}

	return rowData, true
}

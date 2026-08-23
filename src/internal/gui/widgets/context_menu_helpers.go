package widgets

import (
	"fmt"
	"path/filepath"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

type ContextMenuProvider struct {
	treeView  *gtk.TreeView
	listStore *gtk.ListStore
	menu      *gtk.Menu
}

func NewContextMenuProvider(treeView *gtk.TreeView, listStore *gtk.ListStore) *ContextMenuProvider {
	return &ContextMenuProvider{
		treeView:  treeView,
		listStore: listStore,
	}
}

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
			return false
		}

		selection.SelectPath(path)

		if onShowMenu != nil {
			onShowMenu()
		}

		return true
	})
}

func (p *ContextMenuProvider) CreateMenu(fullPathIdx int, columnLabels []string) {
	p.menu = p.buildCopySubmenu(fullPathIdx, columnLabels, nil)
	p.menu.ShowAll()
}

func (p *ContextMenuProvider) CreateMenuWithReveal(
	fullPathIdx int,
	columnLabels []string,
	onReveal func(fullPath string),
) {
	revealItem, _ := gtk.MenuItemNewWithLabel("Show in Explorer")
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

	separator, _ := gtk.SeparatorMenuItemNew()

	prepend := []gtk.IMenuItem{revealItem, separator}
	p.menu = p.buildCopySubmenu(fullPathIdx, columnLabels, prepend)
	p.menu.ShowAll()
}

func (p *ContextMenuProvider) CreateMenuWithExportItem(exportLabel string, onExport func(), columnIndices []int, columnLabels []string) {
	menu, _ := gtk.MenuNew()

	exportItem, _ := gtk.MenuItemNewWithLabel(exportLabel)
	exportItem.Connect("activate", func() {
		if onExport != nil {
			onExport()
		}
	})
	menu.Append(exportItem)

	separator, _ := gtk.SeparatorMenuItemNew()
	menu.Append(separator)

	for i, idx := range columnIndices {
		label := columnLabels[i]
		copyItem, _ := gtk.MenuItemNewWithLabel(fmt.Sprintf("Copy %s", label))
		copyItem.Connect("activate", func() {
			p.copyColumnValue(idx, nil)
		})
		menu.Append(copyItem)
	}

	menu.ShowAll()
	p.menu = menu
}

func (p *ContextMenuProvider) buildCopySubmenu(fullPathIdx int, columnLabels []string, prepend []gtk.IMenuItem) *gtk.Menu {
	menu, _ := gtk.MenuNew()

	for _, item := range prepend {
		menu.Append(item)
	}

	copyItem, _ := gtk.MenuItemNewWithLabel("Copy full path")
	copyItem.Connect("activate", func() {
		p.copyColumnValue(fullPathIdx, nil)
	})
	menu.Append(copyItem)
	copyItem, _ = gtk.MenuItemNewWithLabel("Copy dir path")
	copyItem.Connect("activate", func() {
		p.copyColumnValue(fullPathIdx, filepath.Dir)
	})
	menu.Append(copyItem)

	if len(columnLabels) > 0 {
		separator, _ := gtk.SeparatorMenuItemNew()
		menu.Append(separator)
	}

	for i, label := range columnLabels {
		copyItem, _ := gtk.MenuItemNewWithLabel(fmt.Sprintf("Copy %s", label))
		copyItem.Connect("activate", func() {
			p.copyColumnValue(i, nil)
		})
		menu.Append(copyItem)
	}

	return menu
}

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

		_ = copyToClipboard(value)
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
		return nil, false
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

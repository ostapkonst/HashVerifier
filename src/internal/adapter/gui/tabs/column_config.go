package tabs

import (
	"github.com/gotk3/gotk3/gtk"
)

type ColumnConfig struct {
	titleToName map[string]string
}

func NewGenerateColumnConfig() *ColumnConfig {
	return &ColumnConfig{
		titleToName: map[string]string{
			"Idx":    "idx",
			"Status": "status",
			"Path":   "path",
			"Size":   "size",
			"Hash":   "hash",
			"Note":   "note",
		},
	}
}

func NewVerifyColumnConfig() *ColumnConfig {
	return &ColumnConfig{
		titleToName: map[string]string{
			"Idx":           "idx",
			"Path":          "path",
			"Size":          "size",
			"Status":        "status",
			"Hash":          "hash",
			"Expected Hash": "expected_hash",
			"Note":          "note",
		},
	}
}

func (c *ColumnConfig) GetColumnOrder(treeView *gtk.TreeView) []string {
	return getColumnOrder(treeView, c.titleToName)
}

func (c *ColumnConfig) ApplyColumnOrder(treeView *gtk.TreeView, order []string) {
	if len(order) == 0 {
		return
	}

	applyColumnOrder(treeView, order, c.titleToName)
}

func (c *ColumnConfig) GetSortState(treeView *gtk.TreeView) (string, gtk.SortType) {
	return getSortState(treeView, c.titleToName)
}

func (c *ColumnConfig) ApplySortState(treeView *gtk.TreeView, columnName string, order gtk.SortType) {
	applySortState(treeView, columnName, order, c.titleToName)
}

func getColumnOrder(treeView *gtk.TreeView, titleToName map[string]string) []string {
	columns := treeView.GetColumns()
	result := make([]string, 0)

	for l := columns; l != nil; l = l.Next() {
		col, ok := l.Data().(*gtk.TreeViewColumn)
		if !ok {
			continue
		}

		name := getColumnTitle(col, titleToName)
		if name != "" {
			result = append(result, name)
		}
	}

	return result
}

func applyColumnOrder(treeView *gtk.TreeView, order []string, titleToName map[string]string) {
	if len(order) == 0 {
		return
	}

	columns := treeView.GetColumns()
	columnMap := make(map[string]*gtk.TreeViewColumn)

	for l := columns; l != nil; l = l.Next() {
		col, ok := l.Data().(*gtk.TreeViewColumn)
		if !ok {
			continue
		}

		name := getColumnTitle(col, titleToName)
		if name != "" {
			columnMap[name] = col
		}
	}

	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		if col, ok := columnMap[name]; ok {
			treeView.MoveColumnAfter(col, nil)
		}
	}
}

func getColumnTitle(col *gtk.TreeViewColumn, titleToName map[string]string) string {
	title := col.GetTitle()
	if name, ok := titleToName[title]; ok {
		return name
	}

	return ""
}

func getSortState(treeView *gtk.TreeView, titleToName map[string]string) (string, gtk.SortType) {
	columns := treeView.GetColumns()
	for l := columns; l != nil; l = l.Next() {
		col, ok := l.Data().(*gtk.TreeViewColumn)
		if !ok {
			continue
		}

		if col.GetSortIndicator() {
			name := getColumnTitle(col, titleToName)
			if name != "" {
				sortOrder := col.GetSortOrder()
				return name, sortOrder
			}
		}
	}

	return "", gtk.SORT_ASCENDING
}

func applySortState(treeView *gtk.TreeView, columnName string, order gtk.SortType, titleToName map[string]string) {
	if columnName == "" {
		return
	}

	model, _ := treeView.GetModel()

	listStore, ok := model.(*gtk.ListStore)
	if !ok {
		return
	}

	columns := treeView.GetColumns()
	for l := columns; l != nil; l = l.Next() {
		col, ok := l.Data().(*gtk.TreeViewColumn)
		if !ok {
			continue
		}

		name := getColumnTitle(col, titleToName)
		if name == columnName {
			listStore.SetSortColumnId(col.GetSortColumnID(), order)
			col.SetSortIndicator(true)
			col.SetSortOrder(order)

			return
		}
	}
}

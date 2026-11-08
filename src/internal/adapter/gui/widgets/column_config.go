package widgets

import (
	"github.com/gotk3/gotk3/gtk"
)

// ColumnConfig binds a TreeView's display columns to stable internal names so order and
// sort state can be persisted across runs. The title-to-name map is keyed by the header
// title shown to the user, so the map and the glade titles must stay in sync — a translated
// title would not match.
type ColumnConfig struct {
	titleToName map[string]string
}

// NewGenerateColumnConfig maps the Generate tab's display columns to stable internal names.
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

// NewVerifyColumnConfig maps the Verify tab's display columns to stable internal names.
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

// NewDiscoverColumnConfig maps the Discover tab's display columns to stable internal names.
func NewDiscoverColumnConfig() *ColumnConfig {
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

// GetColumnOrder translates the TreeView's current column order to stable internal names so
// persistence survives column re-titling; columns without a mapping are skipped.
func (c *ColumnConfig) GetColumnOrder(treeView *gtk.TreeView) []string {
	return getColumnOrder(treeView, c.titleToName)
}

// ApplyColumnOrder reorders the TreeView's columns to match order (no-op on empty input).
func (c *ColumnConfig) ApplyColumnOrder(treeView *gtk.TreeView, order []string) {
	if len(order) == 0 {
		return
	}

	applyColumnOrder(treeView, order, c.titleToName)
}

// GetSortState finds the column with the sort indicator and reports its stable name plus
// order; returns "" + SORT_ASCENDING when no column has the indicator set.
func (c *ColumnConfig) GetSortState(treeView *gtk.TreeView) (string, gtk.SortType) {
	return getSortState(treeView, c.titleToName)
}

// ApplySortState sets the sort indicator on the column named columnName (no-op when columnName is "").
func (c *ColumnConfig) ApplySortState(treeView *gtk.TreeView, columnName string, order gtk.SortType) {
	applySortState(treeView, columnName, order, c.titleToName)
}

// SuspendSort captures the TreeView's sort state, hides the headers' sort indicators, disables
// header clicks, and switches the model to UNSORTED so bulk row insertion skips the per-row
// resort. The returned restore reinstates the exact captured state; both must run on the GTK thread.
func (c *ColumnConfig) SuspendSort(treeView *gtk.TreeView) func() {
	model, err := treeView.GetModel()
	if err != nil {
		MustWidget("TreeView", "ColumnConfig.SuspendSort:GetModel", err)
	}

	listStore, ok := model.(*gtk.ListStore)
	if !ok {
		return func() {}
	}

	sortName, sortOrder := getSortState(treeView, c.titleToName)

	type columnState struct {
		column    *gtk.TreeViewColumn
		clickable bool
	}

	var states []columnState

	columns := treeView.GetColumns()
	for l := columns; l != nil; l = l.Next() {
		col, ok := l.Data().(*gtk.TreeViewColumn)
		if !ok {
			continue
		}

		states = append(states, columnState{column: col, clickable: col.GetClickable()})

		// The model change below already clears the indicator via sort-column-changed, but doing it
		// explicitly keeps the hidden state deterministic when the model was already unsorted.
		col.SetSortIndicator(false)
		col.SetClickable(false)
	}

	listStore.SetSortColumnId(gtk.SORT_COLUMN_UNSORTED, gtk.SORT_ASCENDING)

	return func() {
		for _, s := range states {
			s.column.SetClickable(s.clickable)
		}

		if sortName != "" {
			c.ApplySortState(treeView, sortName, sortOrder)
		}
	}
}

// ResetColumnWidths unsets any manual fixed width and recomputes every column to its minimal
// content/header width, so a fresh run starts from the same narrow layout as at startup instead
// of keeping widths grown by a previous run. Must be called on the GTK thread.
func ResetColumnWidths(treeView *gtk.TreeView) {
	columns := treeView.GetColumns()

	for l := columns; l != nil; l = l.Next() {
		col, ok := l.Data().(*gtk.TreeViewColumn)
		if !ok {
			continue
		}

		col.SetFixedWidth(-1)
	}

	treeView.ColumnsAutosize()
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

	model, err := treeView.GetModel()
	if err != nil {
		MustWidget("TreeView", "applySortState:GetModel", err)
	}

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

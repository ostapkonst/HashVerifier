package app

import (
	"github.com/gotk3/gotk3/gtk"
	"github.com/rs/zerolog/log"

	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
)

// TabManager owns notebook tab reordering and current-page persistence around settings.Window.TabOrder and CurrentPage.
type TabManager struct {
	notebook *gtk.Notebook
	window   *gtk.Window
	settings *settings.Settings
}

// NewTabManager wires a TabManager to its notebook, parent window, and settings store.
func NewTabManager(notebook *gtk.Notebook, window *gtk.Window, settings *settings.Settings) *TabManager {
	return &TabManager{
		notebook: notebook,
		window:   window,
		settings: settings,
	}
}

// GetTabOrder returns the current visual order of notebook tabs by their widget name.
func (tm *TabManager) GetTabOrder() []string {
	var order []string

	nPages := tm.notebook.GetNPages()
	for i := range nPages {
		child, err := tm.notebook.GetNthPage(i)
		if err != nil {
			continue
		}

		widget, ok := child.(*gtk.Box)
		if !ok {
			continue
		}

		name, err := widget.GetName()
		if err == nil && name != "" {
			order = append(order, name)
		}
	}

	return order
}

// ApplyTabOrder reorders notebook pages to match settings.Window.TabOrder and persists the result.
func (tm *TabManager) ApplyTabOrder() {
	order := tm.settings.Window.TabOrder
	if len(order) == 0 {
		return
	}

	nPages := tm.notebook.GetNPages()
	pageMap := make(map[string]*gtk.Box)

	for i := range nPages {
		child, err := tm.notebook.GetNthPage(i)
		if err != nil {
			continue
		}

		widget, ok := child.(*gtk.Box)
		if !ok {
			continue
		}

		name, err := widget.GetName()
		if err == nil && name != "" {
			pageMap[name] = widget
		}
	}

	for i, name := range order {
		if child, ok := pageMap[name]; ok {
			tm.notebook.ReorderChild(child, i)
		}
	}
}

// ApplyCurrentPage selects the tab persisted in settings.Window.CurrentPage, clamped to the available range.
func (tm *TabManager) ApplyCurrentPage() {
	tm.ApplySelectedPage(tm.settings.Window.CurrentPage)
}

// ApplySelectedPage selects page but silently ignores out-of-range indices so persisted settings can never crash the UI.
func (tm *TabManager) ApplySelectedPage(page int) {
	if page >= 0 && page < tm.notebook.GetNPages() {
		tm.notebook.SetCurrentPage(page)
	}
}

// GetTabNumberByName returns the notebook index of the page whose widget is named name, or -1 when no match exists.
func (tm *TabManager) GetTabNumberByName(name string) int {
	nPages := tm.notebook.GetNPages()
	for i := range nPages {
		child, err := tm.notebook.GetNthPage(i)
		if err != nil {
			continue
		}

		widget, ok := child.(*gtk.Box)
		if !ok {
			continue
		}

		widgetName, err := widget.GetName()
		if err == nil && widgetName == name {
			return i
		}
	}

	return -1
}

// ConnectReorderHandler wires the page-reordered signal so user-initiated tab drags are persisted to settings.
func (tm *TabManager) ConnectReorderHandler() {
	tm.notebook.Connect("page-reordered", func() {
		if tm.window.InDestruction() {
			return
		}

		tm.settings.Window.TabOrder = tm.GetTabOrder()

		tm.settings.Window.CurrentPage = tm.notebook.GetCurrentPage()
		if err := tm.settings.Save(); err != nil {
			log.Error().Err(err).Msg("Failed to save tab order")
		}
	})
}

// ConnectSwitchHandler wires the switch-page signal to persist the new current page index.
func (tm *TabManager) ConnectSwitchHandler() {
	tm.notebook.Connect(
		"switch-page",
		func(
			_ any,
			_ any,
			pageNum uint,
		) {
			if tm.window.InDestruction() {
				return
			}

			tm.settings.Window.CurrentPage = int(pageNum)
			if err := tm.settings.Save(); err != nil {
				log.Error().Err(err).Msg("Failed to save current page")
			}
		},
	)
}

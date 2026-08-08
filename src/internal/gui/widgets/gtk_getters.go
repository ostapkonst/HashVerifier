package widgets

import (
	"embed"
	"fmt"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

//go:embed glade/*
var assets embed.FS

const (
	uiMain    = "glade/main.glade"
	uiFavIcon = "glade/favicon.ico"
)

func GetMainIcon() (*gdk.Pixbuf, error) {
	uiContent, err := assets.ReadFile(uiFavIcon)
	if err != nil {
		return nil, fmt.Errorf("failed to read UI file: %w", err)
	}

	pixbuf, err := gdk.PixbufNewFromDataOnly(uiContent)
	if err != nil {
		return nil, fmt.Errorf("failed to create pixbuf: %w", err)
	}

	return pixbuf, nil
}

func GetMainForm() (*gtk.Builder, error) {
	builder, err := gtk.BuilderNew()
	if err != nil {
		return nil, fmt.Errorf("failed to create builder: %w", err)
	}

	uiContent, err := assets.ReadFile(uiMain)
	if err != nil {
		return nil, fmt.Errorf("failed to read UI file: %w", err)
	}

	if err := builder.AddFromString(string(uiContent)); err != nil {
		return nil, fmt.Errorf("failed to parse UI: %w", err)
	}

	return builder, nil
}

func GetMainWindow(builder *gtk.Builder) (*gtk.Window, error) {
	obj, err := builder.GetObject("main_window")
	if err != nil {
		return nil, fmt.Errorf("failed to get main window: %w", err)
	}

	window, ok := obj.(*gtk.Window)
	if !ok {
		return nil, fmt.Errorf("object is not a GtkWindow")
	}

	return window, nil
}

func GetButton(builder *gtk.Builder, id string) *gtk.Button {
	button, err := func() (*gtk.Button, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get button %s: %w", id, err)
		}

		button, ok := obj.(*gtk.Button)
		if !ok {
			return nil, fmt.Errorf("object %s is not a Button", id)
		}

		return button, nil
	}()
	if err != nil {
		panic(err)
	}

	return button
}

func GetEntry(builder *gtk.Builder, id string) *gtk.Entry {
	entry, err := func() (*gtk.Entry, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get entry %s: %w", id, err)
		}

		entry, ok := obj.(*gtk.Entry)
		if !ok {
			return nil, fmt.Errorf("object %s is not an Entry", id)
		}

		return entry, nil
	}()
	if err != nil {
		panic(err)
	}

	return entry
}

func GetListStore(builder *gtk.Builder, id string) *gtk.ListStore {
	listStore, err := func() (*gtk.ListStore, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get list store %s: %w", id, err)
		}

		store, ok := obj.(*gtk.ListStore)
		if !ok {
			return nil, fmt.Errorf("object %s is not a ListStore", id)
		}

		return store, nil
	}()
	if err != nil {
		panic(err)
	}

	return listStore
}

func GetComboBoxText(builder *gtk.Builder, id string) *gtk.ComboBoxText {
	comboBox, err := func() (*gtk.ComboBoxText, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get combo box text %s: %w", id, err)
		}

		combo, ok := obj.(*gtk.ComboBoxText)
		if !ok {
			return nil, fmt.Errorf("object %s is not a ComboBoxText", id)
		}

		return combo, nil
	}()
	if err != nil {
		panic(err)
	}

	return comboBox
}

func GetLabel(builder *gtk.Builder, id string) *gtk.Label {
	label, err := func() (*gtk.Label, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get label %s: %w", id, err)
		}

		label, ok := obj.(*gtk.Label)
		if !ok {
			return nil, fmt.Errorf("object %s is not a Label", id)
		}

		return label, nil
	}()
	if err != nil {
		panic(err)
	}

	return label
}

func GetNotebook(builder *gtk.Builder, id string) *gtk.Notebook {
	notebook, err := func() (*gtk.Notebook, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get notebook %s: %w", id, err)
		}

		notebook, ok := obj.(*gtk.Notebook)
		if !ok {
			return nil, fmt.Errorf("object %s is not a Notebook", id)
		}

		return notebook, nil
	}()
	if err != nil {
		panic(err)
	}

	return notebook
}

func GetProgressBar(builder *gtk.Builder, id string) *gtk.ProgressBar {
	bar, err := func() (*gtk.ProgressBar, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get progress bar %s: %w", id, err)
		}

		bar, ok := obj.(*gtk.ProgressBar)
		if !ok {
			return nil, fmt.Errorf("object %s is not a ProgressBar", id)
		}

		return bar, nil
	}()
	if err != nil {
		panic(err)
	}

	return bar
}

func GetCheckButton(builder *gtk.Builder, id string) *gtk.CheckButton {
	button, err := func() (*gtk.CheckButton, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get check button %s: %w", id, err)
		}

		button, ok := obj.(*gtk.CheckButton)
		if !ok {
			return nil, fmt.Errorf("object %s is not a CheckButton", id)
		}

		return button, nil
	}()
	if err != nil {
		panic(err)
	}

	return button
}

func GetTreeView(builder *gtk.Builder, id string) *gtk.TreeView {
	tree, err := func() (*gtk.TreeView, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get tree view %s: %w", id, err)
		}

		treeView, ok := obj.(*gtk.TreeView)
		if !ok {
			return nil, fmt.Errorf("object %s is not a TreeView", id)
		}

		return treeView, nil
	}()
	if err != nil {
		panic(err)
	}

	return tree
}

func GetGrid(builder *gtk.Builder, id string) *gtk.Grid {
	grid, err := func() (*gtk.Grid, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get grid %s: %w", id, err)
		}

		grid, ok := obj.(*gtk.Grid)
		if !ok {
			return nil, fmt.Errorf("object %s is not a Grid", id)
		}

		return grid, nil
	}()
	if err != nil {
		panic(err)
	}

	return grid
}

func GetFrame(builder *gtk.Builder, id string) *gtk.Frame {
	frame, err := func() (*gtk.Frame, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get frame %s: %w", id, err)
		}

		frame, ok := obj.(*gtk.Frame)
		if !ok {
			return nil, fmt.Errorf("object %s is not a Frame", id)
		}

		return frame, nil
	}()
	if err != nil {
		panic(err)
	}

	return frame
}

func GetCellRendererToggle(builder *gtk.Builder, id string) *gtk.CellRendererToggle {
	renderer, err := func() (*gtk.CellRendererToggle, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get cell renderer toggle %s: %w", id, err)
		}

		r, ok := obj.(*gtk.CellRendererToggle)
		if !ok {
			return nil, fmt.Errorf("object %s is not a CellRendererToggle", id)
		}

		return r, nil
	}()
	if err != nil {
		panic(err)
	}

	return renderer
}

func GetSearchEntry(builder *gtk.Builder, id string) *gtk.SearchEntry {
	entry, err := func() (*gtk.SearchEntry, error) {
		obj, err := builder.GetObject(id)
		if err != nil {
			return nil, fmt.Errorf("failed to get search entry %s: %w", id, err)
		}

		e, ok := obj.(*gtk.SearchEntry)
		if !ok {
			return nil, fmt.Errorf("object %s is not a SearchEntry", id)
		}

		return e, nil
	}()
	if err != nil {
		panic(err)
	}

	return entry
}

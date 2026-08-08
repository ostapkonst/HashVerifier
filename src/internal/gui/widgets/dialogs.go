package widgets

import (
	"fmt"

	"github.com/gotk3/gotk3/gtk"
)

func ShowError(parent *gtk.Window, title, message string) {
	dialog := gtk.MessageDialogNew(
		parent,
		gtk.DIALOG_MODAL,
		gtk.MESSAGE_ERROR,
		gtk.BUTTONS_OK,
		"%s", message,
	)
	defer dialog.Destroy()

	dialog.SetTitle(title)
	dialog.Run()
}

func ShowConfirmOverwriteDialog(parent *gtk.Window, filename string) bool {
	dialog := gtk.MessageDialogNew(
		parent,
		gtk.DIALOG_MODAL,
		gtk.MESSAGE_WARNING,
		gtk.BUTTONS_YES_NO,
		"File already exists:\n%s\n\nDo you want to overwrite it?", filename,
	)
	defer dialog.Destroy()

	dialog.SetTitle("File Already Exists")

	return dialog.Run() == gtk.RESPONSE_YES
}

func SelectDirectoryDialog(parent *gtk.Window, title, folder string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER,
		"_Open",
		gtk.RESPONSE_ACCEPT,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
	)
	if err != nil {
		ShowError(parent, "Select Directory Error", fmt.Sprintf("Failed to create select directory dialog: %v", err))
		return "", false
	}
	defer dialog.Destroy()

	dialog.SetCurrentFolder(folder)

	if dialog.Run() == gtk.RESPONSE_ACCEPT {
		dir := dialog.GetFilename()
		return dir, true
	}

	return "", false
}

func OpenFileDialog(parent *gtk.Window, title, path string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"_Open",
		gtk.RESPONSE_ACCEPT,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
	)
	if err != nil {
		ShowError(parent, "Open File Error", fmt.Sprintf("Failed to create open file dialog: %v", err))
		return "", false
	}
	defer dialog.Destroy()

	folder, filename := SplitPath(path)
	dialog.SetCurrentFolder(folder)
	dialog.SetCurrentName(filename)

	AddFileFilters(dialog, filename)

	if dialog.Run() == gtk.RESPONSE_ACCEPT {
		file := dialog.GetFilename()
		return file, true
	}

	return "", false
}

func OpenAnyFileDialog(parent *gtk.Window, title, path string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"_Open",
		gtk.RESPONSE_ACCEPT,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
	)
	if err != nil {
		ShowError(parent, "Open File Error", fmt.Sprintf("Failed to create open file dialog: %v", err))
		return "", false
	}
	defer dialog.Destroy()

	folder, filename := SplitPath(path)
	dialog.SetCurrentFolder(folder)
	dialog.SetCurrentName(filename)

	if dialog.Run() == gtk.RESPONSE_ACCEPT {
		file := dialog.GetFilename()
		return file, true
	}

	return "", false
}

func SaveFileDialog(parent *gtk.Window, title, path, ext string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"_Save",
		gtk.RESPONSE_ACCEPT,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
	)
	if err != nil {
		ShowError(parent, "Save File Error", fmt.Sprintf("Failed to create save file dialog: %v", err))
		return "", false
	}
	defer dialog.Destroy()

	folder, filename := SplitPath(path)
	dialog.SetCurrentFolder(folder)
	dialog.SetCurrentName(filename)

	if dialog.Run() == gtk.RESPONSE_ACCEPT {
		file := dialog.GetFilename()
		return file, true
	}

	return "", false
}

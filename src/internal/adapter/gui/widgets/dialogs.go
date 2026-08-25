package widgets

import (
	"fmt"
	"strings"

	"github.com/gotk3/gotk3/gtk"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
)

// ShowError displays a blocking modal error dialog with an OK button and returns once the user dismisses it.
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

// ShowConfirmOverwriteDialog asks the user to overwrite an existing file and returns true only on Yes.
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

// SelectDirectoryDialog opens a modal folder picker seeded at folder; the bool is true only when the user accepts.
func SelectDirectoryDialog(parent *gtk.Window, title, folder string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SELECT_FOLDER,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
		"_Open",
		gtk.RESPONSE_ACCEPT,
	)
	if err != nil {
		MustWidget("FileChooserDialog", "SelectDirectoryDialog", err)
	}
	defer dialog.Destroy()

	dialog.SetCurrentFolder(folder)

	if dialog.Run() == gtk.RESPONSE_ACCEPT {
		dir := dialog.GetFilename()
		return dir, true
	}

	return "", false
}

// OpenFileDialog seeds the picker at path and infers algorithm filters from its extension; bool signals accept.
func OpenFileDialog(parent *gtk.Window, title, path string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
		"_Open",
		gtk.RESPONSE_ACCEPT,
	)
	if err != nil {
		MustWidget("FileChooserDialog", "OpenFileDialog", err)
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

// OpenAnyFileDialog seeds the picker at path with no extension filter; bool signals accept.
func OpenAnyFileDialog(parent *gtk.Window, title, path string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
		"_Open",
		gtk.RESPONSE_ACCEPT,
	)
	if err != nil {
		MustWidget("FileChooserDialog", "OpenAnyFileDialog", err)
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

// SaveFileDialog seeds the save-as picker at path; ext narrows the filter to that algorithm when known.
func SaveFileDialog(parent *gtk.Window, title, path, ext string) (string, bool) {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		title,
		parent,
		gtk.FILE_CHOOSER_ACTION_SAVE,
		"_Cancel",
		gtk.RESPONSE_CANCEL,
		"_Save",
		gtk.RESPONSE_ACCEPT,
	)
	if err != nil {
		MustWidget("FileChooserDialog", "SaveFileDialog", err)
	}
	defer dialog.Destroy()

	folder, filename := SplitPath(path)
	dialog.SetCurrentFolder(folder)
	dialog.SetCurrentName(filename)

	if ext != "" {
		if a, algoErr := algorithm.AlgorithmFromExtension(ext); algoErr == nil {
			pattern := "*" + a.Extension()

			filter, err := gtk.FileFilterNew()
			if err != nil {
				MustWidget("FileFilter", "SaveFileDialog", err)
			}

			filter.SetName(fmt.Sprintf("%s (%s)", a.DisplayName(), pattern))
			filter.AddPattern(pattern)
			filter.AddPattern(strings.ToUpper(pattern))
			dialog.AddFilter(filter)
			dialog.SetFilter(filter)
		}
	}

	if dialog.Run() == gtk.RESPONSE_ACCEPT {
		file := dialog.GetFilename()
		return file, true
	}

	return "", false
}

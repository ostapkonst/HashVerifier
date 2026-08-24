package widgets

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
	"github.com/ostapkonst/HashVerifier/internal/appmeta"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
)

func ChangeFileExtension(filename, ext string) string {
	if filename == "" {
		return ""
	}

	filenameWithoutExtension := strings.TrimSuffix(filename, filepath.Ext(filename))

	return filenameWithoutExtension + ext
}

func GenChecksumFilename(directory, ext string) string {
	if IsRootPath(directory) {
		return ""
	}

	return directory + ext
}

func GenChecksumFilenameFlat(directory, ext string) string {
	return filepath.Join(directory, "checksums"+ext)
}

func IsRootPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.Dir(clean) == clean
}

func SplitPath(fullPath string) (directory, filename string) {
	if fullPath == "" {
		return "", ""
	}

	directory = filepath.Dir(fullPath)
	filename = filepath.Base(fullPath)

	return directory, filename
}

func ListStoreString(listStore *gtk.ListStore, iter *gtk.TreeIter, col int) (string, bool) {
	val, err := listStore.GetValue(iter, col)
	if err != nil {
		return "", false
	}

	goVal, err := val.GoValue()
	if err != nil {
		return "", false
	}

	str, ok := goVal.(string)
	if !ok {
		return "", false
	}

	return str, true
}

func ListStoreBool(listStore *gtk.ListStore, iter *gtk.TreeIter, col int) (bool, bool) {
	val, err := listStore.GetValue(iter, col)
	if err != nil {
		return false, false
	}

	goVal, err := val.GoValue()
	if err != nil {
		return false, false
	}

	b, ok := goVal.(bool)
	if !ok {
		return false, false
	}

	return b, true
}

func AddFileFilters(dialog *gtk.FileChooserDialog, filename string) {
	allSupported, allFiles := addAlgorithmFilters(dialog)

	if _, err := algorithm.AlgorithmFromExtension(filename); err == nil || filename == "" {
		dialog.SetFilter(allSupported)
	} else {
		dialog.SetFilter(allFiles)
	}
}

func addAlgorithmFilters(dialog *gtk.FileChooserDialog) (allSupported, allFiles *gtk.FileFilter) {
	algorithms := algorithm.SupportedAlgorithms

	allSupported, _ = gtk.FileFilterNew()
	allSupported.SetName(
		fmt.Sprintf("All Supported Files (%d algorithms)", len(algorithms)),
	)

	for _, a := range algorithms {
		pattern := "*" + a.Extension()
		allSupported.AddPattern(pattern)
		allSupported.AddPattern(strings.ToUpper(pattern))
	}

	dialog.AddFilter(allSupported)

	for _, a := range algorithms {
		pattern := "*" + a.Extension()
		filter, _ := gtk.FileFilterNew()
		filter.SetName(fmt.Sprintf("%s (%s)", a.DisplayName(), pattern))
		filter.AddPattern(pattern)
		filter.AddPattern(strings.ToUpper(pattern))
		dialog.AddFilter(filter)
	}

	allFiles, _ = gtk.FileFilterNew()
	allFiles.SetName("All Files (*.*)")
	allFiles.AddPattern("*")

	dialog.AddFilter(allFiles)

	return allSupported, allFiles
}

func ShowAboutDialog(parent *gtk.Window, icon *gdk.Pixbuf) {
	about, err := gtk.AboutDialogNew()
	if err != nil {
		ShowError(parent, "About Error", fmt.Sprintf("Failed to create about dialog: %v", err))
		return
	}
	defer about.Destroy()

	gtkVersion := fmt.Sprintf("%d.%d.%d", gtk.GetMajorVersion(), gtk.GetMinorVersion(), gtk.GetMicroVersion())

	about.SetTransientFor(parent)
	about.SetModal(true)
	about.SetLogo(icon)
	about.SetProgramName(appmeta.Name)
	about.SetVersion(appmeta.Version)
	about.SetWebsiteLabel(appmeta.Link)
	about.SetComments("A cross-platform application for generating and validating file checksums using multiple cryptographic hash algorithms.\n\nGTK Version: " + gtkVersion)
	about.SetCopyright("© Ostap Konstantinov")
	about.Run()
}

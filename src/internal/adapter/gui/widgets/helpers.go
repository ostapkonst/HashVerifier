package widgets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	"github.com/ostapkonst/HashVerifier/internal/appmeta"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
)

// ChangeFileExtension returns filename with its existing extension replaced by ext (or appended when none).
func ChangeFileExtension(filename, ext string) string {
	if filename == "" {
		return ""
	}

	filenameWithoutExtension := strings.TrimSuffix(filename, filepath.Ext(filename))

	return filenameWithoutExtension + ext
}

// GenChecksumFilename returns directory+ext for hierarchical output; "" when directory is a root path.
func GenChecksumFilename(directory, ext string) string {
	if IsRootPath(directory) {
		return ""
	}

	return directory + ext
}

// GenChecksumFilenameFlat returns directory/checksums+ext so flat output writes inside the source directory.
func GenChecksumFilenameFlat(directory, ext string) string {
	return filepath.Join(directory, "checksums"+ext)
}

// IsRootPath reports whether path is a filesystem root (e.g., "/" or "C:\").
func IsRootPath(path string) bool {
	clean := filepath.Clean(path)
	return filepath.Dir(clean) == clean
}

// isDirPath reports whether path is a directory, following symlinks so a symlink-to-dir is
// rendered as a directory; an unreadable or missing path counts as a file.
func isDirPath(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.IsDir()
}

// ScrollListToTop moves the TreeView's viewport back to the first row. Called after the final
// re-sort: the reorder moves the row the view was anchored on, and TreeView restores its stale
// top_row asynchronously, so setting the adjustment directly is a no-op (its value is often
// already 0) and the view jumps back mid-list. ScrollToCell registers scroll_to_path/scroll_to_point,
// which TreeView's deferred validation honors. No-op when the tree is not realized or empty.
func ScrollListToTop(treeView *gtk.TreeView) {
	if !treeView.GetRealized() {
		return
	}

	model, err := treeView.GetModel()
	if err != nil {
		return
	}

	listStore, ok := model.(*gtk.ListStore)
	if !ok {
		return
	}

	if _, ok := listStore.GetIterFirst(); !ok {
		return
	}

	path, err := gtk.TreePathNewFromString("0")
	if err != nil {
		return
	}

	treeView.ScrollToCell(path, nil, true, 0.0, 0.0)
}

// SplitPath returns the directory and base filename; both "" when fullPath is "" (lets callers skip empty checks).
func SplitPath(fullPath string) (directory, filename string) {
	if fullPath == "" {
		return "", ""
	}

	directory = filepath.Dir(fullPath)
	filename = filepath.Base(fullPath)

	return directory, filename
}

// ListStoreString returns the string value at (iter, col), or ("", false) on type mismatch or read error.
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

// ListStoreBool returns the bool value at (iter, col), or (false, false) on type mismatch or read error.
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

// AddFileFilters populates the chooser with per-algorithm + all-supported + all-files
// filters and selects the best match for filename.
func AddFileFilters(dialog *gtk.FileChooserDialog, filename string) {
	allSupported, allFiles := addAlgorithmFilters(dialog)

	if _, err := algorithm.AlgorithmFromExtension(filename); err == nil || filename == "" {
		dialog.SetFilter(allSupported)
	} else {
		dialog.SetFilter(allFiles)
	}
}

// addAlgorithmFilters adds one filter per supported algorithm plus an all-supported and all-files filter to the chooser.
func addAlgorithmFilters(dialog *gtk.FileChooserDialog) (allSupported, allFiles *gtk.FileFilter) {
	algorithms := algorithm.SupportedAlgorithms

	allSupported, err := gtk.FileFilterNew()
	if err != nil {
		MustWidget("FileFilter", "addAlgorithmFilters:allSupported", err)
	}

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

		filter, err := gtk.FileFilterNew()
		if err != nil {
			MustWidget("FileFilter", "addAlgorithmFilters:perAlgo", err)
		}

		filter.SetName(fmt.Sprintf("%s (%s)", a.DisplayName(), pattern))
		filter.AddPattern(pattern)
		filter.AddPattern(strings.ToUpper(pattern))
		dialog.AddFilter(filter)
	}

	allFiles, err = gtk.FileFilterNew()
	if err != nil {
		MustWidget("FileFilter", "addAlgorithmFilters:allFiles", err)
	}

	allFiles.SetName("All Files (*.*)")
	allFiles.AddPattern("*")

	dialog.AddFilter(allFiles)

	return allSupported, allFiles
}

// ShowAboutDialog opens the modal About dialog populated from appmeta and the current GTK version.
func ShowAboutDialog(parent *gtk.Window, icon *gdk.Pixbuf) {
	about, err := gtk.AboutDialogNew()
	if err != nil {
		MustWidget("AboutDialog", "ShowAboutDialog", err)
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

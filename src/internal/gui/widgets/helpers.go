package widgets

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/internal/header"
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
	if IsRootPath(directory) {
		return ""
	}

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

func AddFileFilters(dialog *gtk.FileChooserDialog, filename string) {
	supportedFiles := [][]string{
		{"CRC-32", "*.sfv"},
		{"MD4", "*.md4"},
		{"MD5", "*.md5"},
		{"SHA1", "*.sha1"},
		{"SHA256", "*.sha256"},
		{"SHA384", "*.sha384"},
		{"SHA512", "*.sha512"},
		{"SHA3-256", "*.sha3-256"},
		{"SHA3-384", "*.sha3-384"},
		{"SHA3-512", "*.sha3-512"},
		{"BLAKE3", "*.blake3"},
		{"XXH3", "*.xxh3"},
		{"XXH128", "*.xxh128"},
	}
	filterAllSupported, _ := gtk.FileFilterNew()
	filterAllSupported.SetName(
		fmt.Sprintf("All Supported Files (%d algorithms)", len(supportedFiles)),
	)

	for _, pattern := range supportedFiles {
		filterAllSupported.AddPattern(pattern[1])
		filterAllSupported.AddPattern(strings.ToUpper(pattern[1]))
	}

	dialog.AddFilter(filterAllSupported)

	for _, pattern := range supportedFiles {
		filter, _ := gtk.FileFilterNew()
		filter.SetName(fmt.Sprintf("%s (%s)", pattern[0], pattern[1]))
		filter.AddPattern(pattern[1])
		filter.AddPattern(strings.ToUpper(pattern[1]))
		dialog.AddFilter(filter)
	}

	filterAny, _ := gtk.FileFilterNew()
	filterAny.SetName("All Files (*.*)")
	filterAny.AddPattern("*")

	dialog.AddFilter(filterAny)

	if _, err := checksum.AlgorithmFromExtension(filename); err == nil || filename == "" {
		dialog.SetFilter(filterAllSupported)
	} else {
		dialog.SetFilter(filterAny)
	}
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
	about.SetProgramName(header.Name)
	about.SetVersion(header.Version)
	about.SetWebsiteLabel(header.Link)
	about.SetComments("A cross-platform application for generating and validating file checksums using multiple cryptographic hash algorithms.\n\nGTK Version: " + gtkVersion)
	about.SetCopyright("© Ostap Konstantinov")
	about.Run()
}

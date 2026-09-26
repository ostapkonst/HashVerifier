//go:build windows

package fs

import "golang.org/x/sys/windows"

// hiddenComponent reports whether the final path element carries the Windows
// FILE_ATTRIBUTE_HIDDEN flag. A lookup failure counts as not hidden; an unreadable entry is
// surfaced by the walker's own permission handling.
func hiddenComponent(path string) bool {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}

	attrs, err := windows.GetFileAttributes(name)
	if err != nil {
		return false
	}

	return attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0
}

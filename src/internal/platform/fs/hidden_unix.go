//go:build !windows

package fs

import (
	"path/filepath"
	"strings"
)

// hiddenComponent reports whether the final path element follows the Unix convention:
// a name starting with a dot. "." and ".." cannot appear as a real element here.
func hiddenComponent(path string) bool {
	return strings.HasPrefix(filepath.Base(path), ".")
}

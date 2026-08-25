package hashfn

import (
	"runtime"
	"strings"
)

// PathsEqual compares resolved absolute paths; case-insensitive on Windows (NTFS default), case-sensitive elsewhere.
func PathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

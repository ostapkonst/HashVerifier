package hashfn

import (
	"runtime"
	"strings"
)

// PathsEqual reports whether two already-resolved absolute paths refer to the same file;
// case-insensitive on Windows and macOS, where the default filesystem is case-preserving but
// case-insensitive, and case-sensitive elsewhere.
func PathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

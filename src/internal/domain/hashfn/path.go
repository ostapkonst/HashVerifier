package hashfn

import (
	"runtime"
	"strings"
)

// PathsEqual compares two absolute paths with case-insensitive semantics on Windows (NTFS default) and case-sensitive everywhere else. Caller is responsible for passing resolved absolute paths.
func PathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

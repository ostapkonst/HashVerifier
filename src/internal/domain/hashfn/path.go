package hashfn

import (
	"runtime"
	"strings"
)

// PathsEqual reports whether two already-resolved absolute paths refer to the same file; case-insensitive on Windows, case-sensitive elsewhere.
func PathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

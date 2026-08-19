package calculator

import (
	"runtime"
	"strings"
)

// PathsEqual compares two absolute filesystem paths.
//
// On Windows (NTFS is case-insensitive by default) the comparison is
// case-insensitive via strings.EqualFold. On other platforms — where the
// underlying filesystems are case-sensitive — paths are compared byte-for-byte
// to preserve the expected semantics (e.g. /data/Foo and /data/foo are
// distinct files on ext4).
//
// Callers should pass already-resolved absolute paths (see filepath.Abs);
// the helper does not normalize separators or strip prefixes (e.g. the long-path
// prefix \\?\) — that is intentionally out of scope.
func PathsEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}

	return a == b
}

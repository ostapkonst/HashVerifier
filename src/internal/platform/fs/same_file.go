package fs

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// SameFile reports whether a and b refer to the same on-disk entity. Paths are resolved through
// Abs → EvalSymlinks (best effort) → platform case-folding, so short/long Windows names, "./" prefixes
// and symlink aliases compare equal. An Abs failure is reported; an EvalSymlinks failure silently falls
// back to the Abs form, keeping the comparison useful on broken or permission-restricted paths.
func SameFile(a, b string) (bool, error) {
	ra, err := canonicalPath(a)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", a, err)
	}

	rb, err := canonicalPath(b)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", b, err)
	}

	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(ra, rb), nil
	}

	return ra == rb, nil
}

func canonicalPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}

	return abs, nil
}

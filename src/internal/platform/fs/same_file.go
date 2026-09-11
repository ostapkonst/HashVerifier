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

// SeenPaths tracks which file paths have already been processed, deduplicating
// entries that resolve to the same on-disk entity (via symlinks, "./" prefixes,
// etc.). Add returns false if path duplicates a previously-added one.
type SeenPaths map[string]struct{}

// Add records path and reports whether it is new (true) or duplicates a previously
// recorded path that resolves to the same on-disk entity (false). When a SameFile
// comparison fails (broken symlink, permission, etc.), the error is returned and
// path is treated as new — better to process a possibly-duplicate path than to
// silently drop a legitimate one.
func (s SeenPaths) Add(path string) (bool, error) {
	for prior := range s {
		same, err := SameFile(prior, path)
		if err != nil {
			return true, err
		}

		if same {
			return false, nil
		}
	}

	s[path] = struct{}{}

	return true, nil
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

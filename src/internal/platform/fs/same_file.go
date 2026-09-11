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

// SeenPaths tracks which file paths have already been processed. Add does an exact-string
// comparison only — no normalization, no filesystem access. Symlink aliases, "./foo" vs "foo",
// and case-only differences are NOT detected. For symlink-correct dedup use fs.SameFile directly.
type SeenPaths map[string]struct{}

// Add reports whether path is new (true) or duplicates a previously-added entry (false).
func (s SeenPaths) Add(path string) bool {
	if _, ok := s[path]; ok {
		return false
	}

	s[path] = struct{}{}

	return true
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

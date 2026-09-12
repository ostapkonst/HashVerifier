package fs

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// CaseFold returns s lowercased on case-insensitive filesystems (Windows, macOS) and
// unchanged on case-sensitive ones (Linux). It mirrors actual filesystem behavior — not an
// arbitrary transformation — and is the right helper when comparing user-supplied paths or
// names against entries that the OS would treat as the same identity.
func CaseFold(s string) string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(s)
	}

	return s
}

// CaseEqualFold reports whether a and b are equal under the host filesystem's case
// sensitivity — case-insensitive on Windows/macOS, case-sensitive on Linux.
func CaseEqualFold(a, b string) bool {
	return CaseFold(a) == CaseFold(b)
}

// SameFile reports whether a and b refer to the same on-disk entity. Paths are resolved
// through Abs (to canonicalize relative input and short/long Windows names) → EvalSymlinks
// (so symlink aliases compare equal) → CaseEqualFold (so Windows and macOS, whose
// filesystems are case-insensitive by default, treat Foo and foo as the same file). An
// Abs failure is reported; an EvalSymlinks failure silently falls back to the Abs form,
// keeping the comparison useful on broken or permission-restricted paths.
func SameFile(a, b string) (bool, error) {
	ra, err := canonicalPath(a)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", a, err)
	}

	rb, err := canonicalPath(b)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", b, err)
	}

	return CaseEqualFold(ra, rb), nil
}

// SeenPaths tracks which file paths have already been processed. Add normalizes via
// filepath.Clean (OS-aware resolution of "./", "..", and redundant slashes) and lowercases
// on case-insensitive filesystems (Windows, macOS), mirroring actual filesystem behavior.
// Symlink aliases are NOT detected — see SameFile for that, at the cost of a stat per comparison.
type SeenPaths map[string]struct{}

// Add reports whether path is new (true) or duplicates a previously-added entry (false).
func (s SeenPaths) Add(path string) bool {
	key := CaseFold(filepath.Clean(path))

	if _, ok := s[key]; ok {
		return false
	}

	s[key] = struct{}{}

	return true
}

// canonicalPath returns p resolved to an absolute path with symlinks followed. When
// EvalSymlinks fails (broken symlink, permission error, etc.), the unresolved absolute path
// is returned — Symlink comparison falls back to comparing the un-followed targets, which
// still produces a useful answer for the common "two paths pointing at the same file" case.
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

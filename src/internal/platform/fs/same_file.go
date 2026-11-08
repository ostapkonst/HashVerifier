package fs

import (
	"fmt"
	"os"
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

// SameFile reports whether a and b refer to the same on-disk entity. When both paths can
// be stat'd, identity is decided by os.SameFile (device + inode), which also catches hard
// links and symlink aliases. Otherwise (e.g. an output file that does not exist yet) it
// falls back to canonical paths: Abs → EvalSymlinks → CaseEqualFold, so Windows and macOS,
// whose filesystems are case-insensitive by default, treat Foo and foo as the same file.
// An Abs failure is reported; an EvalSymlinks failure silently falls back to the Abs form,
// keeping the fallback comparison useful on broken or permission-restricted paths.
func SameFile(a, b string) (bool, error) {
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)

	if errA == nil && errB == nil {
		return os.SameFile(infoA, infoB), nil
	}

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

// PathKey returns the canonical key of path: filepath.Clean applied, then case-folded on
// case-insensitive filesystems (Windows, macOS), mirroring actual filesystem behavior. Two
// paths that differ only in case are the same file there, so they must share a key; on
// case-sensitive filesystems (Linux) the case is kept and the paths stay distinct.
func PathKey(path string) string {
	return CaseFold(filepath.Clean(path))
}

// PathIndex tracks the canonical paths already processed and assigns each of them a stable
// index in first-seen order. Paths are canonicalized with PathKey (OS-aware resolution of
// "./", "..", and redundant slashes, plus case folding on case-insensitive filesystems),
// mirroring actual filesystem behavior.
// Symlink aliases are NOT detected — see SameFile for that, at the cost of a stat per comparison.
type PathIndex map[string]int

// Index returns the index assigned to path, allocating the next one when the path is seen
// for the first time; the bool reports whether the path was new.
func (p PathIndex) Index(path string) (int, bool) {
	key := PathKey(path)

	if idx, ok := p[key]; ok {
		return idx, false
	}

	idx := len(p)
	p[key] = idx

	return idx, true
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

// exclude.go implements path-based exclusion for checksum generation.
//
// A Matcher is built from a list of relative paths (files and directories)
// and answers whether a given rel-path should be skipped during generation.
// Directories are matched prefix-wise: excluding "build/" also excludes
// "build/app.exe" and "build/sub/out.log", but not "build-tools/x"
// (component-aware boundary).
//
// All paths are normalized to forward slashes and cleaned via filepath.Clean,
// so the same Matcher works regardless of the host OS path separator.
// Backslashes (Windows-style) are translated to forward slashes before
// cleaning, enabling cross-platform round-trip of exclusion lists.
package exclude

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ErrExcludedByUser is the sentinel error attached to GenerateResult when a
// file is skipped because it matched an exclude entry. It is classified as
// GenSkipped (not GenFailed) by the generator.
var ErrExcludedByUser = fmt.Errorf("excluded by user")

// IsExcludedError reports whether err is the user-exclusion sentinel.
func IsExcludedError(err error) bool {
	return err != nil && err == ErrExcludedByUser
}

// Matcher decides whether a relative path is excluded from checksum generation.
//
// Exclusions are exact relative paths selected by the user (no wildcards).
// A directory entry excludes every file nested under it. All paths are
// normalized to forward slashes so that matchers behave the same regardless
// of the host OS path separator.
//
// A nil *Matcher is safe to call — IsExcluded returns false and Count
// returns 0.
type Matcher struct {
	// files holds exact rel-paths of excluded files (normalized, no
	// trailing slash). Lookup is O(1).
	files map[string]struct{}

	// dirPrefixes holds normalized rel-paths of excluded directories,
	// each with a trailing "/" appended. A rel-path is excluded if it
	// equals the prefix without the slash (the dir itself) or starts
	// with the prefix (nested file). Scan is O(D) where D is the number
	// of excluded directories.
	dirPrefixes []string
}

// NewMatcher builds a Matcher from relative paths.
//
// Each entry is normalized to forward-slash form via normalize. A trailing
// slash or backslash on the input marks the entry as a directory prefix;
// otherwise it is treated as an exact file path. Empty and "." entries are
// silently ignored.
//
// Callers should pass already-resolved relative paths (see filepath.Rel)
// — the Matcher does not validate that paths are relative or exist on disk.
func NewMatcher(relPaths []string) *Matcher {
	m := &Matcher{
		files:       make(map[string]struct{}, len(relPaths)),
		dirPrefixes: make([]string, 0, len(relPaths)),
	}

	for _, p := range relPaths {
		if p == "" {
			continue
		}

		isDir := strings.HasSuffix(p, "/") || strings.HasSuffix(p, `\`)

		clean := normalize(p)
		if clean == "" || clean == "." {
			continue
		}

		if isDir {
			m.dirPrefixes = append(m.dirPrefixes, clean+"/")
		} else {
			m.files[clean] = struct{}{}
		}
	}

	return m
}

// IsExcluded reports whether relPath should be excluded from generation.
//
// relPath is normalized to forward slashes before comparison. A path matches
// if:
//   - it equals one of the exact file entries, OR
//   - it is or is nested under one of the excluded directory prefixes
//     (component-aware: "build/" does not match "build-tools/x").
//
// A nil receiver returns false. Empty and "." inputs return false.
func (m *Matcher) IsExcluded(relPath string) bool {
	if m == nil || len(m.files) == 0 && len(m.dirPrefixes) == 0 {
		return false
	}

	clean := normalize(relPath)
	if clean == "" || clean == "." {
		return false
	}

	if _, ok := m.files[clean]; ok {
		return true
	}

	for _, prefix := range m.dirPrefixes {
		// Match the directory itself (prefix without trailing slash).
		if clean == strings.TrimSuffix(prefix, "/") {
			return true
		}

		// Match any file nested under the directory.
		// prefix already ends with "/", so HasPrefix is component-aware:
		// "build/" does not match "build-tools/x".
		if strings.HasPrefix(clean, prefix) {
			return true
		}
	}

	return false
}

// Count returns the total number of exclusion entries (files + directories).
// Returns 0 for a nil receiver.
func (m *Matcher) Count() int {
	if m == nil {
		return 0
	}

	return len(m.files) + len(m.dirPrefixes)
}

// normalize converts a relative path to canonical forward-slash form
// regardless of the host OS path separator.
//
// Backslashes (Windows-style) are replaced with forward slashes first, so
// that filepath.Clean treats them as separators on every platform (on Linux,
// filepath.Clean does not interpret backslashes as separators). The result
// is then passed through filepath.ToSlash to handle any OS-specific
// separators that Clean may have introduced.
func normalize(p string) string {
	if p == "" {
		return ""
	}

	p = strings.ReplaceAll(p, `\`, "/")

	return filepath.ToSlash(filepath.Clean(p))
}

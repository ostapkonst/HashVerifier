// Package exclude evaluates user-supplied rel-paths against new paths during generate; trailing '/' marks a directory.
package exclude

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ErrExcludedByUser is the sentinel error returned when a file matches an exclude entry; classified as GenSkipped.
var ErrExcludedByUser = fmt.Errorf("excluded by user")

// IsExcludedError lets callers classify user-exclusion failures without type-asserting against ErrExcludedByUser.
func IsExcludedError(err error) bool {
	return err != nil && err == ErrExcludedByUser
}

// Matcher evaluates rel-paths against a list of excluded entries; a nil receiver is safe.
type Matcher struct {
	// files: exact rel-paths of excluded files (normalized). O(1) lookup.
	files map[string]struct{}

	// dirPrefixes: normalized rel-paths of excluded directories with trailing "/". O(D) scan.
	dirPrefixes []string
}

// NewMatcher builds a Matcher from rel-paths; trailing "/" or "\" on input marks a directory prefix.
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

// IsExcluded reports whether relPath is excluded; a nil receiver returns false.
// Directory matches are component-aware ("build/" excludes "build/x" but not "build-tools/x").
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

// normalize canonicalizes a rel-path to forward-slash form; filepath.Clean on Linux doesn't treat '\\' as a separator, so replace it first.
func normalize(p string) string {
	if p == "" {
		return ""
	}

	p = strings.ReplaceAll(p, `\`, "/")

	return filepath.ToSlash(filepath.Clean(p))
}

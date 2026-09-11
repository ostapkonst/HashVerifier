// Package exclude evaluates user-supplied path rules against files discovered during generate.
// A Rule is built once via ParseRule and stored after filepath.Clean (OS-aware "./" and ".."
// resolution; no separator conversion). All consumers — CLI, GUI dialog, generator — share
// that single normalization so every platform observes identical matching semantics. A rule matches
// a top-level tree entry by name (the entry itself, and its contents if it's a directory).
// Component-aware matching means "build" excludes "build/x" but not "build-tools/x". Case
// handling follows the host platform's default filesystem: rules and walked paths are
// case-folded on Windows and macOS (case-insensitive filesystems), kept literal on Linux.
// Note: backslash is a regular filename character on Linux/macOS and a path separator on
// Windows; users on Windows should use "\" (or both) in their rules to match the walked path.
package exclude

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
)

// ErrExcludedByUser is the sentinel error returned when a file matches an exclude rule; classified as GenSkipped.
var ErrExcludedByUser = errors.New("excluded by user")

// ErrInvalidRule reports an empty or degenerate exclude entry (e.g. "", ".", "..").
var ErrInvalidRule = errors.New("invalid exclude rule")

// IsExcludedError lets callers classify user-exclusion failures without type-asserting against ErrExcludedByUser.
func IsExcludedError(err error) bool {
	return err != nil && errors.Is(err, ErrExcludedByUser)
}

// Rule is one immutable exclude pattern in canonical form: cleaned, no leading "./" or ".".
// The fields are unexported to keep the invariant: the only way to obtain a valid Rule is
// ParseRule. One path component cannot be both a file and a directory on any supported
// filesystem, so a single rule covers either case at the same path level.
type Rule struct {
	path string
}

// ParseRule builds a Rule from raw. Only filepath.Clean is applied — no separator
// conversion (no \→/, no ToSlash): user input is taken as-is after OS-aware "./" and ".."
// resolution. Absolute paths are rejected on every platform. Case handling is delegated to
// MatchesCanonical via fs.CaseFold, so rules and walked paths are compared case-insensitively
// only on Windows and macOS.
func ParseRule(raw string) (Rule, error) {
	p := filepath.Clean(raw)

	if p == "." || p == ".." {
		return Rule{}, ErrInvalidRule
	}

	if filepath.IsAbs(p) {
		return Rule{}, ErrInvalidRule
	}

	return Rule{path: p}, nil
}

// String returns the canonical textual form.
func (r Rule) String() string {
	return r.path
}

// RuleStrings renders rules in their canonical form, for logging and display.
func RuleStrings(rules []Rule) []string {
	out := make([]string, len(rules))

	for i, r := range rules {
		out[i] = r.String()
	}

	return out
}

// Matches reports whether relPath (raw, from the walker) satisfies the rule.
func (r Rule) Matches(relPath string) bool {
	return r.MatchesCanonical(normalizeRel(relPath))
}

// MatchesCanonical is Matches for an already-canonicalized rel path (per normalizeRel); avoids
// re-cleaning when the caller holds a Matcher with many rules.
func (r Rule) MatchesCanonical(clean string) bool {
	if clean == "" {
		return false
	}

	clean = fs.CaseFold(clean)
	target := fs.CaseFold(r.path)

	// Component-aware prefix: "build" matches "build" and "build/x", but not "build-tools/x".
	return clean == target || strings.HasPrefix(clean, target+"/")
}

// Matcher evaluates canonical Rules against rel paths discovered during a walk; a nil receiver is safe.
type Matcher struct {
	rules []Rule // canonical rules in construction order; case-folding is applied to the query path per GOOS at match time
}

// NewMatcher builds a Matcher from canonical Rules.
func NewMatcher(rules []Rule) *Matcher {
	return &Matcher{rules: rules}
}

// IsExcluded reports whether relPath is excluded; a nil receiver returns false.
func (m *Matcher) IsExcluded(relPath string) bool {
	if m == nil || len(m.rules) == 0 {
		return false
	}

	clean := normalizeRel(relPath)
	if clean == "" {
		return false
	}

	for _, r := range m.rules {
		if r.MatchesCanonical(clean) {
			return true
		}
	}

	return false
}

// normalizeRel cleans a walked rel path via filepath.Clean only — no separator conversion,
// no case folding (that happens inside MatchesCanonical). A leading "./" cannot survive
// filepath.Clean; "." maps to "" (degenerate, rejected by the caller).
func normalizeRel(p string) string {
	p = filepath.Clean(p)

	if p == "." {
		return ""
	}

	return p
}

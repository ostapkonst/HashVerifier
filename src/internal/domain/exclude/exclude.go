// Package exclude evaluates user-supplied path rules against files discovered during generate.
// A Rule is built once via ParseRule and stored in its canonical form (forward slashes, cleaned);
// all consumers — CLI, GUI dialog, generator — share that single normalization so every
// platform observes identical matching semantics. A rule matches a top-level tree entry by
// name (the entry itself, and its contents if it's a directory). Component-aware matching
// means "build" excludes "build/x" but not "build-tools/x". A trailing slash on input is
// accepted for backward compatibility and stripped during normalization. Case handling follows
// the host platform's default filesystem: insensitive on Windows and macOS, sensitive elsewhere.
package exclude

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrExcludedByUser is the sentinel error returned when a file matches an exclude rule; classified as GenSkipped.
var ErrExcludedByUser = errors.New("excluded by user")

// ErrInvalidRule reports an empty or degenerate exclude entry (e.g. "", ".", "/", "..").
var ErrInvalidRule = errors.New("invalid exclude rule")

// IsExcludedError lets callers classify user-exclusion failures without type-asserting against ErrExcludedByUser.
func IsExcludedError(err error) bool {
	return err != nil && errors.Is(err, ErrExcludedByUser)
}

// Rule is one immutable exclude pattern in canonical form: slash-separated, cleaned, no leading
// slash or "./". The fields are unexported to keep the invariant: the only way to obtain a
// valid Rule is ParseRule. One path component cannot be both a file and a directory on any
// supported filesystem, so a single rule covers either case at the same path level.
type Rule struct {
	path string
}

// ParseRule normalizes raw into a canonical Rule. Leading-slash absolute paths (e.g. "/x") are rejected on every
// platform; Windows drive-letter paths (e.g. "C:\x") are rejected on Windows only. Trailing slashes and backslashes
// are accepted on input and stripped during normalization so users who type `build/` and `build` get the same rule.
func ParseRule(raw string) (Rule, error) {
	// Backslash folding mirrors checksum_parser.fixPathSeparator and is safe on POSIX because the
	// hashing guard (hashfn.ErrPathContainsInvalidSeparator) skips backslash names before they reach
	// the matcher — the checksum format cannot represent them; see docs/USAGE.md skipped-files note.
	p := strings.ReplaceAll(raw, `\`, "/")
	if strings.HasPrefix(p, "/") || filepath.IsAbs(p) {
		return Rule{}, ErrInvalidRule
	}

	p = filepath.ToSlash(filepath.Clean(p))

	// filepath.Clean never returns "" (empty input becomes "."), and it strips leading "./" itself,
	// so only "." and ".." need an explicit reject.
	if p == "." || p == ".." {
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

	clean = caseFold(clean)
	target := caseFold(r.path)

	// Component-aware prefix: "build" matches "build" and "build/x", but not "build-tools/x".
	return clean == target || strings.HasPrefix(clean, target+"/")
}

// Matcher evaluates canonical Rules against rel paths discovered during a walk; a nil receiver is safe.
type Matcher struct {
	rules []Rule // canonical rules in construction order; case-folded per GOOS inside MatchesCanonical
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

// normalizeRel is the single path cleaner shared by Rule parsing and Matcher lookups;
// the backslash fold here follows the same format-contract rationale documented in ParseRule.
// Returns "" for degenerate inputs: filepath.Clean maps "" and "." to ".", which is the only
// outcome we must catch; a leading "./" cannot survive filepath.Clean.
func normalizeRel(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = filepath.ToSlash(filepath.Clean(p))

	if p == "." {
		return ""
	}

	return p
}

// caseFold lowercases s on filesystems that are typically case-insensitive
// (Windows and macOS) so exclude rules match the on-disk casing; other platforms keep s unchanged.
func caseFold(s string) string {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToLower(s)
	}

	return s
}

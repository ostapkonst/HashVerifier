// Package exclude evaluates user-supplied path rules against files discovered during generate.
// A Rule is built once via ParseRule and stored in its canonical form (forward slashes, cleaned);
// all consumers — CLI, GUI dialog, generator — share that single normalization so every
// platform observes identical matching semantics. Directory rules carry a trailing slash;
// matching is component-aware ("build/" excludes "build/x" but not "build-tools/x").
// Case handling follows the host platform's default filesystem: insensitive on Windows
// and macOS, sensitive elsewhere.
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

// Rule is one immutable exclude pattern in canonical form: slash-separated, cleaned, no leading slash
// or "./"; directories are marked by isDir rather than a stored trailing separator. The fields are
// unexported to keep the invariant: the only way to obtain a valid Rule is ParseRule.
type Rule struct {
	path  string // canonical rel path without trailing slash, e.g. "build/secret"
	isDir bool
}

// ParseRule normalizes raw into a canonical Rule; the dir marker is a trailing "/" or "\" captured
// BEFORE cleaning (filepath.Clean strips it), so "build /" stays a file rule while "build/" is a dir.
// Absolute paths ("/x", "C:\x") are rejected — exclude rules are relative by contract.
func ParseRule(raw string) (Rule, error) {
	// Backslash folding mirrors checksum_parser.fixPathSeparator and is safe on POSIX because the
	// hashing guard (hashfn.ErrPathContainsInvalidSeparator) skips backslash names before they reach
	// the matcher — the checksum format cannot represent them; see docs/USAGE.md skipped-files note.
	p := strings.ReplaceAll(raw, `\`, "/")
	if strings.HasPrefix(p, "/") || filepath.IsAbs(p) {
		return Rule{}, ErrInvalidRule
	}

	isDir := strings.HasSuffix(p, "/")
	p = filepath.ToSlash(filepath.Clean(p))

	// filepath.Clean never returns "" (empty input becomes "."), and it strips leading "./" itself,
	// so only "." and ".." need an explicit reject.
	if p == "." || p == ".." {
		return Rule{}, ErrInvalidRule
	}

	return Rule{path: p, isDir: isDir}, nil
}

// String returns the canonical textual form; directory rules keep a trailing "/".
func (r Rule) String() string {
	if r.isDir {
		return r.path + "/"
	}

	return r.path
}

// IsDir reports whether the rule targets a directory.
func (r Rule) IsDir() bool {
	return r.isDir
}

// Path returns the canonical rel path without any trailing separator.
func (r Rule) Path() string {
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

// Matches reports whether relPath (raw, from the walker) satisfies the rule; the directory itself
// is also matched ("build/" excludes "build" as well as "build/x" — walker never emits dir entries,
// but the equivalence keeps the semantics obvious and keeps CLI flags idempotent).
func (r Rule) Matches(relPath string) bool {
	return r.MatchesCanonical(normalizeRel(relPath))
}

// MatchesCanonical is Matches for an already-canonicalized rel path (per normalizeRel); avoids
// re-cleaning when the caller holds a Matcher with many dir rules.
func (r Rule) MatchesCanonical(clean string) bool {
	if clean == "" {
		return false
	}

	clean = caseFold(clean)
	target := caseFold(r.path)

	if r.isDir {
		// Directory itself or anything nested under it; the "/" suffix keeps matches component-aware.
		return clean == target || strings.HasPrefix(clean, target+"/")
	}

	return clean == target
}

// Matcher evaluates canonical Rules against rel paths discovered during a walk; a nil receiver is safe.
type Matcher struct {
	files map[string]struct{} // canonical file paths (case-folded per GOOS)
	dirs  []Rule              // directory rules, matched via prefix logic in Rule.Matches
}

// NewMatcher builds a Matcher from canonical Rules.
func NewMatcher(rules []Rule) *Matcher {
	m := &Matcher{
		files: make(map[string]struct{}, len(rules)),
		dirs:  make([]Rule, 0, len(rules)),
	}

	for _, r := range rules {
		if r.isDir {
			m.dirs = append(m.dirs, r)
		} else {
			m.files[caseFold(r.path)] = struct{}{}
		}
	}

	return m
}

// IsExcluded reports whether relPath is excluded; a nil receiver returns false.
func (m *Matcher) IsExcluded(relPath string) bool {
	if m == nil || len(m.files) == 0 && len(m.dirs) == 0 {
		return false
	}

	clean := normalizeRel(relPath)
	if clean == "" {
		return false
	}

	if _, ok := m.files[caseFold(clean)]; ok {
		return true
	}

	for _, dir := range m.dirs {
		if dir.MatchesCanonical(clean) {
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

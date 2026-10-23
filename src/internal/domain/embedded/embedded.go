// Package embedded parses the bracketed CRC-32 checksum convention in file names
// (e.g. filename [1a2b3c4d].ext) for the discover flow.
package embedded

import (
	"path/filepath"
	"regexp"
	"strings"
)

// checksumRe matches a name carrying a bracketed 8-hex CRC-32; the leading group is greedy so the
// last bracketed checksum wins when several appear. The separating space before the bracket is
// optional.
var checksumRe = regexp.MustCompile(`^(.*)\[([0-9a-fA-F]{8})\](.*)$`)

// Split returns name's stem, an embedded CRC-32 (when present), and the remainder that follows it.
// For a name without a checksum the remainder is its extension (e.g. go.sum -> stem "go", rest
// ".sum"). Only one separating space is removed before the bracket: a trailing space in the base
// name is legitimate on Linux.
func Split(name string) (stem, checksum, rest string, ok bool) {
	if m := checksumRe.FindStringSubmatch(name); m != nil {
		return strings.TrimSuffix(m[1], " "), m[2], m[3], true
	}

	ext := filepath.Ext(name)

	stem = strings.TrimSuffix(name, ext)
	if stem == "" {
		return name, "", "", false
	}

	return stem, "", ext, false
}

// FilterMode selects which files a discovery scan considers.
type FilterMode int

const (
	// FilterAll considers every file.
	FilterAll FilterMode = iota
	// FilterWithChecksum considers only names that embed a CRC-32.
	FilterWithChecksum
	// FilterWithoutChecksum considers only names that do not embed a CRC-32.
	FilterWithoutChecksum
)

// String returns the filter's canonical name for logging.
func (m FilterMode) String() string {
	switch m {
	case FilterWithChecksum:
		return "with_checksum"
	case FilterWithoutChecksum:
		return "without_checksum"
	default:
		return "all"
	}
}

// FilterModeFromString parses the canonical name produced by String; ok is false for unknown values.
func FilterModeFromString(s string) (FilterMode, bool) {
	switch s {
	case "with_checksum":
		return FilterWithChecksum, true
	case "without_checksum":
		return FilterWithoutChecksum, true
	case "all":
		return FilterAll, true
	default:
		return FilterAll, false
	}
}

// Accepts reports whether name passes the filter.
func (m FilterMode) Accepts(name string) bool {
	_, _, _, ok := Split(name)

	switch m {
	case FilterWithChecksum:
		return ok
	case FilterWithoutChecksum:
		return !ok
	default:
		return true
	}
}

// Package embedded parses and rewrites the bracketed CRC-32 checksum convention in file names
// (e.g. filename [1a2b3c4d].ext), shared by the discover and rename flows.
package embedded

import (
	"path/filepath"
	"regexp"
	"strings"
)

// checksumRe matches a name carrying a bracketed 8-hex CRC-32; the leading group is greedy so the
// last bracketed checksum wins when several appear. The separating space before the bracket is
// optional when parsing but always emitted by Join.
var checksumRe = regexp.MustCompile(`^(.*)\[([0-9a-fA-F]{8})\](.*)$`)

// Split returns name's stem, an embedded CRC-32 (when present), and the remainder that follows it.
// For a name without a checksum the remainder is its extension, so Join always rebuilds the
// canonical stem [checksum]ext layout (e.g. go.sum -> stem "go", rest ".sum"). Only one separating
// space is removed before the bracket: a trailing space in the base name is legitimate on Linux.
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

// Join renders stem, a space, a bracketed checksum, and rest back into one name.
func Join(stem, checksum, rest string) string {
	return stem + " [" + checksum + "]" + rest
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

// RenameMode selects how names carrying a CRC-32 are rewritten.
type RenameMode int

const (
	// RenameAdd inserts the computed CRC-32 into names that do not carry one.
	RenameAdd RenameMode = iota
	// RenameRemove strips an embedded CRC-32 from names that carry one.
	RenameRemove
	// RenameUpdate replaces an embedded CRC-32 with the computed one.
	RenameUpdate
)

// String returns the rename mode's canonical name for logging.
func (m RenameMode) String() string {
	switch m {
	case RenameRemove:
		return "remove"
	case RenameUpdate:
		return "update"
	default:
		return "add"
	}
}

// RenameModeFromString parses the canonical name produced by String; ok is false for unknown values.
func RenameModeFromString(s string) (RenameMode, bool) {
	switch s {
	case "remove":
		return RenameRemove, true
	case "update":
		return RenameUpdate, true
	case "add":
		return RenameAdd, true
	default:
		return RenameAdd, false
	}
}

// PlanRename applies mode to name using checksum and reports the resulting name; changed is false
// when the name is already in the desired shape. Checksum comparison is case-insensitive: a
// case-only rename would be rejected as "target already exists" on case-insensitive filesystems.
func PlanRename(name, checksum string, mode RenameMode) (newName string, changed bool) {
	stem, embeddedChecksum, rest, hasChecksum := Split(name)

	switch mode {
	case RenameAdd:
		if hasChecksum {
			return name, false
		}

		newName = Join(stem, checksum, rest)
	case RenameRemove:
		if !hasChecksum {
			return name, false
		}

		newName = stem + rest
	case RenameUpdate:
		if !hasChecksum || strings.EqualFold(embeddedChecksum, checksum) {
			return name, false
		}

		newName = Join(stem, checksum, rest)
	default:
		return name, false
	}

	return newName, newName != name
}

// RenameCandidate is a file discovered by a scan, paired with the CRC-32 computed from its contents.
type RenameCandidate struct {
	Path     string
	Checksum string
}

// RenameAction is one planned rename: a file's current path and its new path.
type RenameAction struct {
	From string
	To   string
}

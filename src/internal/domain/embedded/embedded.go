// Package embedded parses and rewrites the bracketed CRC-32 checksum convention in file names
// (e.g. filename [1a2b3c4d].ext) for the discover flow.
package embedded

import (
	"path/filepath"
	"regexp"
	"strings"
)

// checksumRe matches a name carrying a bracketed 8-hex CRC-32; the leading group is greedy so the
// last bracketed checksum wins when several appear.
var checksumRe = regexp.MustCompile(`^(.*)\[([0-9a-fA-F]{8})\](.*)$`)

// Checksum returns the CRC-32 embedded in name, spelled as the name spells it, and reports
// whether the name carries one at all.
func Checksum(name string) (string, bool) {
	m := checksumRe.FindStringSubmatch(name)
	if m == nil {
		return "", false
	}

	return m[2], true
}

// InsertChecksum appends a bracketed checksum before the final extension; a dotfile such as
// ".hidden" has no extension for this purpose, so the checksum follows its whole name. The
// caller must ensure name carries no checksum yet.
func InsertChecksum(name, checksum string) string {
	ext := filepath.Ext(name)
	if ext == name {
		ext = ""
	}

	return strings.TrimSuffix(name, ext) + " [" + checksum + "]" + ext
}

// ReplaceChecksum swaps the last bracketed checksum in place, preserving the surrounding text;
// it reports false and returns name unchanged when the name carries no checksum.
func ReplaceChecksum(name, checksum string) (string, bool) {
	m := checksumRe.FindStringSubmatch(name)
	if m == nil {
		return name, false
	}

	return m[1] + "[" + checksum + "]" + m[3], true
}

// RemoveChecksum deletes the last bracketed checksum and the spaces separating it from the stem;
// it reports false and returns name unchanged when the name carries no checksum.
func RemoveChecksum(name string) (string, bool) {
	m := checksumRe.FindStringSubmatch(name)
	if m == nil {
		return name, false
	}

	return strings.TrimRight(m[1], " ") + m[3], true
}

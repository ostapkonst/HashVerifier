// Package embedded parses the bracketed CRC-32 checksum convention in file names
// (e.g. filename [1a2b3c4d].ext) for the discover flow.
package embedded

import "regexp"

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

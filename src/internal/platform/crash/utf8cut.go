package crash

import "unicode/utf8"

// utf8Cut returns the index to cut s at so that s[:cut] is valid UTF-8 and the cut
// always makes progress: it never lands inside a rune and never returns 0. The
// result may exceed max by up to 3 bytes when the leading rune is longer than max.
//
// s must be non-empty and max must be smaller than len(s).
func utf8Cut(s string, max int) int {
	for cut := max; cut > 0; cut-- {
		if utf8.RuneStart(s[cut]) {
			return cut
		}
	}

	// No rune boundary within max: the leading rune is longer than max, so emit it
	// whole rather than splitting it.
	_, size := utf8.DecodeRuneInString(s)

	return size
}

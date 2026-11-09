//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package crash

import (
	"strings"
	"unicode/utf8"
)

// reportChunkSize keeps every OS-log message under the classic syslog ~1 KB wire limit and
// Unified Logging's ~1 KB dynamic-content limit; longer reports are split instead of being
// truncated or rejected by the transport.
const reportChunkSize = 900

// chunkUTF8 splits s into pieces of at most max bytes, preferring line boundaries so
// every log entry reads as whole stack frames; it never breaks UTF-8 sequences.
func chunkUTF8(s string, max int) []string {
	var chunks []string

	for len(s) > max {
		cut := strings.LastIndexByte(s[:max], '\n')
		if cut > 0 {
			cut++
		} else {
			cut = max
			for cut > 0 && !utf8.RuneStart(s[cut]) {
				cut--
			}

			if cut == 0 {
				cut = max
			}
		}

		chunks = append(chunks, s[:cut])
		s = s[cut:]
	}

	if s != "" {
		chunks = append(chunks, s)
	}

	return chunks
}

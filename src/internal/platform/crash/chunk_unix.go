//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package crash

import (
	"strings"
)

// reportChunkSize keeps each syslog and Unified Logging message under their ~1 KB
// transport limit; longer reports are split instead of being truncated or rejected
// by the transport.
const reportChunkSize = 900

// chunkUTF8 splits s into pieces of at most max bytes, preferring line boundaries so
// every log entry reads as whole stack frames; it never breaks UTF-8 sequences. A
// piece may exceed max by up to 3 bytes — the length of the longest UTF-8 rune — when
// the leading rune is longer than max. A non-positive max disables splitting.
func chunkUTF8(s string, max int) []string {
	if max <= 0 {
		return []string{s}
	}

	var chunks []string

	for len(s) > max {
		cut := strings.LastIndexByte(s[:max], '\n')
		if cut > 0 {
			cut++
		} else {
			cut = utf8Cut(s, max)
		}

		chunks = append(chunks, s[:cut])
		s = s[cut:]
	}

	if s != "" {
		chunks = append(chunks, s)
	}

	return chunks
}

// Package eol provides the platform-correct line ending ("\r\n" on Windows, "\n" elsewhere).
package eol

import (
	"runtime"
)

// PlatformEOL is the line terminator for files written by HashVerifier on the current OS.
var PlatformEOL = platformEOL()

func platformEOL() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}

	return "\n"
}

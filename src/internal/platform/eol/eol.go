package eol

import (
	"runtime"
)

var PlatformEOL = platformEOL()

func platformEOL() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}

	return "\n"
}

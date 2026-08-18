package eof

import (
	"runtime"
)

var PlatformEOF = getEOF()

func getEOF() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}

	return "\n"
}

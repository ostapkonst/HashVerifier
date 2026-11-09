//go:build darwin

// Package opendocs delivers files opened via Finder/LaunchServices to the GUI on macOS and
// routes AppKit quit requests through the shared graceful shutdown.
package opendocs

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void installOpenDocumentsHandler(void);
*/
import "C"

import (
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

var (
	handlerMu sync.RWMutex
	handler   func(string)
)

// Install registers handler for files opened via Finder ("Open With", Dock, double-click).
// A single open action carrying several files delivers only the first one — the window is
// single-document; the rest are dropped with a log entry. Safe to call more than once.
func Install(h func(string)) {
	handlerMu.Lock()
	handler = h
	handlerMu.Unlock()

	C.installOpenDocumentsHandler()
}

//export goOpenFile
func goOpenFile(cpath *C.char, count C.int) {
	path := C.GoString(cpath)
	if path == "" {
		return
	}

	handlerMu.RLock()
	h := handler
	handlerMu.RUnlock()

	if h == nil {
		log.Warn().Str("path", path).Msg("Open-documents event arrived before the handler was installed, dropping")
		return
	}

	if count > 1 {
		log.Warn().Str("path", path).Int("total", int(count)).Msg("Open action carried multiple files, opening only the first")
	}

	h(path)
}

// goRequestQuit routes AppKit quit requests (Cmd+Q, scripted quit) through the shared
// graceful-shutdown path; the delegate cancels the immediate AppKit termination.
//
//export goRequestQuit
func goRequestQuit() {
	shutdown.GracefulShutdown()
}

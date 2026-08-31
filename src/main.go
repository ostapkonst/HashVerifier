package main

// Workaround for Go 1.23 behavior change regarding Windows Junction Links
// https://go.dev/doc/go1.23#ospkgos
// Only applies when compiling with CGO_ENABLED=1

/*
#include <stdio.h>
#include <stdlib.h>

#ifdef _WIN32
   #define setenv(name, value, overwrite) _putenv_s(name, value)
#endif

__attribute__((constructor))
static void call_init_env() {
    setenv("GODEBUG", "winsymlink=0", 1);
}
*/
import "C"

import (
	"errors"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/inhies/go-bytesize"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	cliadapter "github.com/ostapkonst/HashVerifier/internal/adapter/cli"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	guiapp "github.com/ostapkonst/HashVerifier/internal/adapter/gui/app"
)

func main() {
	bytesize.Format = "%.2f "

	var runErr error
	if isWindows() {
		runErr = runOnWindows(os.Args[1:])
	} else {
		runErr = runOnLinux()
	}

	if runErr == nil {
		return
	}

	var exitErr *base.ExitError
	if errors.As(runErr, &exitErr) {
		if exitErr.Err != nil && !exitErr.Silent {
			log.Error().Err(exitErr.Err).Msg("Application failed")
		}

		os.Exit(exitErr.Code)
	}

	os.Exit(1)
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func runOnWindows(args []string) error {
	log.Logger = zerolog.New(io.Discard)

	// Windows has no CLI mode, so the flag is always false here; HASHVERIFIER_NO_CONFIG still works via env.Bool in gui.Run.
	var runErr error
	if len(args) > 0 {
		runErr = guiapp.Run(args[0], false)
	} else {
		runErr = guiapp.Run("", false)
	}

	if runErr == nil {
		return nil
	}

	var exitErr *base.ExitError
	if errors.As(runErr, &exitErr) {
		return exitErr
	}

	return &base.ExitError{Code: 1, Err: runErr, Silent: true}
}

func runOnLinux() error {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	return cliadapter.Execute()
}

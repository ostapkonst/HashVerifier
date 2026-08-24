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

	if isWindows() {
		if err := runOnWindows(os.Args[1:]); err != nil {
			os.Exit(1)
		}

		return
	}

	if err := runOnLinux(); err != nil {
		var exitErr *base.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.Err != nil && !exitErr.Silent {
				log.Error().Err(exitErr.Err).Msg("Application failed")
			}

			os.Exit(exitErr.Code)
		}

		log.Fatal().Err(err).Msg("Application failed")
	}
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func runOnWindows(args []string) error {
	log.Logger = zerolog.New(io.Discard)

	if len(args) > 0 {
		return guiapp.Run(args[0])
	}

	return guiapp.Run("")
}

func runOnLinux() error {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	return cliadapter.Execute()
}

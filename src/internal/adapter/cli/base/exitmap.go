package base

import (
	"errors"

	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/platform/shutdown"
)

// MapShutdownError translates shutdown-terminated runs into a 130-exit ExitError so shell and CI tooling
// can distinguish a user-initiated cancel from an operation failure. Returns the input unchanged when
// it is neither ErrWaitTimeout nor ErrForceStopped, so it is safe to apply unconditionally to any
// shutdown.Wait() return value. Logs a warning at the point of translation so the user sees the cause
// (timeout vs. forced stop) even when the caller discards the returned error.
func MapShutdownError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, shutdown.ErrWaitTimeout):
		log.Warn().Msg("Operation canceled: shutdown callback timeout")
		return &ExitError{Code: 130, Err: err, Silent: true}
	case errors.Is(err, shutdown.ErrForceStopped):
		log.Warn().Msg("Operation canceled: forced shutdown (second signal)")
		return &ExitError{Code: 130, Err: err, Silent: true}
	}

	return err
}

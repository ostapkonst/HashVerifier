package widgets

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// MustWidget logs a GTK operation failure and panics: continuing past a broken widget is worse than a clean crash.
func MustWidget(widget, op string, err error) {
	log.Error().
		Err(err).
		Str("widget", widget).
		Str("op", op).
		Msg("GTK operation failed; aborting")
	panic(fmt.Sprintf("%s in %s: %v", widget, op, err))
}

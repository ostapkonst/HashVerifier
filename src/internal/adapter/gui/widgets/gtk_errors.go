package widgets

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// MustWidget logs a GTK-related operation failure with structured context
// and panics. Use for widget constructions (and adjacent GTK operations
// like LoadFromData / GetScreen) where failure indicates unrecoverable
// state — continuing with a broken widget is worse than a clean crash.
//
// Consistent with the fail-fast pattern in gtk_getters.go.
func MustWidget(widget, op string, err error) {
	log.Error().
		Err(err).
		Str("widget", widget).
		Str("op", op).
		Msg("GTK operation failed; aborting")
	panic(fmt.Sprintf("%s in %s: %v", widget, op, err))
}

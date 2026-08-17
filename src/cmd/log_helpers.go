package cmd

import (
	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/settings"
)

func logLoadWarnings(s *settings.Settings) {
	for _, w := range s.LoadWarnings() {
		log.Warn().
			Str("field", w.Field).
			Str("invalid_value", w.Value).
			Str("default", w.Default).
			Msg("Invalid settings value, replaced with default")
	}
}

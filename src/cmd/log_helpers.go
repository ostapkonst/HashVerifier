package cmd

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/envutil"
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

func loadAndLog(cmd *cobra.Command) *settings.Settings {
	s, err := settings.Load(loadNoConfig(cmd))
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load settings, using defaults")

		s = settings.DefaultSettings()
	}

	logLoadWarnings(s)

	return s
}

func loadNoConfig(cmd *cobra.Command) bool {
	if cmd.Flags().Changed("no-config") {
		v, _ := cmd.Flags().GetBool("no-config")
		return v
	}

	return envutil.Bool("HASHVERIFIER_NO_CONFIG")
}

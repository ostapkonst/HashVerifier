package base

import (
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/env"
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

// LoadAndLog loads settings honoring the --no-config flag, logs any repairs (LoadWarnings), and falls back to defaults on error.
func LoadAndLog(cmd *cobra.Command) *settings.Settings {
	s, err := settings.Load(LoadNoConfig(cmd))
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load settings, using defaults")

		s = settings.DefaultSettings()
	}

	logLoadWarnings(s)

	return s
}

// LoadForConfig loads settings silently for config-* subcommands. The caller is responsible for reporting errors.
func LoadForConfig(cmd *cobra.Command) (*settings.Settings, error) {
	return settings.Load(LoadNoConfig(cmd))
}

// LoadNoConfig returns true when --no-config was passed or the HASHVERIFIER_NO_CONFIG env var is truthy.
func LoadNoConfig(cmd *cobra.Command) bool {
	if cmd.Flags().Changed("no-config") {
		v, _ := cmd.Flags().GetBool("no-config")
		return v
	}

	return env.Bool("HASHVERIFIER_NO_CONFIG")
}

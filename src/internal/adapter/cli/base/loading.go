package base

import (
	"fmt"

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
// On load failure the repair warnings are lost: logLoadWarnings runs on the fresh DefaultSettings() which has none.
// Returns an error only for the unrecoverable --no-config parse failure; settings-load failure is logged and falls back silently.
func LoadAndLog(cmd *cobra.Command) (*settings.Settings, error) {
	noConfig, err := LoadNoConfig(cmd)
	if err != nil {
		return nil, err
	}

	s, err := settings.Load(noConfig)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load settings, using defaults")

		s = settings.DefaultSettings()
	}

	logLoadWarnings(s)

	return s, nil
}

// LoadForConfig loads settings silently for config-* subcommands. The caller is responsible for reporting errors.
func LoadForConfig(cmd *cobra.Command) (*settings.Settings, error) {
	noConfig, err := LoadNoConfig(cmd)
	if err != nil {
		return nil, err
	}

	return settings.Load(noConfig)
}

// LoadNoConfig returns true when --no-config was passed or the HASHVERIFIER_NO_CONFIG env var is truthy.
// Returns an error when cobra fails to parse the --no-config flag, which should never happen for a registered Bool flag.
func LoadNoConfig(cmd *cobra.Command) (bool, error) {
	if cmd.Flags().Changed("no-config") {
		v, err := cmd.Flags().GetBool("no-config")
		if err != nil {
			return false, fmt.Errorf("internal error reading --no-config flag: %w", err)
		}

		return v, nil
	}

	return env.Bool("HASHVERIFIER_NO_CONFIG"), nil
}

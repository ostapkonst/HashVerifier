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

// LoadAndLog loads settings honoring the --no-config flag, logs any repairs (LoadWarnings),
// and falls back to defaults on error. The fallback carries no repair warnings — they belong
// to the file that failed to load. Returns an error only for the unrecoverable --no-config
// parse failure.
func LoadAndLog(cmd *cobra.Command) (*settings.Settings, error) {
	noConfig, err := LoadNoConfig(cmd)
	if err != nil {
		return nil, err
	}

	var s *settings.Settings

	if noConfig {
		s = settings.NewDefaultsWithWorker(true)
	} else {
		s, err = settings.Load()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load settings, using defaults")

			s = settings.NewDefaultsWithWorker(false)
		}
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

	if noConfig {
		return settings.NewDefaultsWithWorker(true), nil
	}

	return settings.Load()
}

// LoadNoConfig returns true when --no-config was passed as true, or when the flag was not passed
// and the HASHVERIFIER_NO_CONFIG env var is truthy; an explicit --no-config=false wins over the env var.
// Returns an error when cobra fails to parse the --no-config flag, which should never happen for a registered Bool flag.
func LoadNoConfig(cmd *cobra.Command) (bool, error) {
	return FlagBoolOrDefault(cmd, "no-config", env.NoConfig())
}

package cmd

import (
	"fmt"
	"strings"

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

// printRepairs renders a structured "Repairs applied" block to stdout for
// user-facing config-* commands. Empty slice skips printing anything.
// Prints a leading blank line to separate from preceding output.
func printRepairs(warnings []settings.ValidationWarning) {
	if len(warnings) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Repairs applied (%d):\n", len(warnings))
	fmt.Println(strings.Repeat("=", 80))

	for _, w := range warnings {
		fmt.Printf("  %s: %q -> %q\n", w.Field, w.Value, w.Default)
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

// loadForConfig loads settings for config-* subcommands without logging.
// Returns (defaults, err) on parse failure; the caller is expected to
// report the error to the user explicitly (via its own stderr format and
// exit code) instead of relying on zerolog WRN spam.
func loadForConfig(cmd *cobra.Command) (*settings.Settings, error) {
	return settings.Load(loadNoConfig(cmd))
}

func loadNoConfig(cmd *cobra.Command) bool {
	if cmd.Flags().Changed("no-config") {
		v, _ := cmd.Flags().GetBool("no-config")
		return v
	}

	return envutil.Bool("HASHVERIFIER_NO_CONFIG")
}

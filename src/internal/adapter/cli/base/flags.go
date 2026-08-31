package base

import (
	"fmt"

	"github.com/spf13/cobra"
)

// AddOptBoolFlag registers a Bool flag where `--name` (no value) is treated as true via NoOptDefVal.
func AddOptBoolFlag(cmd *cobra.Command, name string, defaultVal bool, usage string) {
	cmd.Flags().Bool(name, defaultVal, usage)
	cmd.Flags().Lookup(name).NoOptDefVal = "true"
}

// AddOptBoolPersistentFlag registers a persistent Bool flag where `--name` (no value) is treated as true via NoOptDefVal;
// persistent flags are inherited by every subcommand.
func AddOptBoolPersistentFlag(rootCmd *cobra.Command, name string, defaultVal bool, usage string) {
	rootCmd.PersistentFlags().Bool(name, defaultVal, usage)
	rootCmd.PersistentFlags().Lookup(name).NoOptDefVal = "true"
}

// FlagBool reads a Bool flag that has no config-file fallback. Wraps the cobra parse error
// with the flag name so callers can surface it as a real ExitError.
func FlagBool(cmd *cobra.Command, name string) (bool, error) {
	v, err := cmd.Flags().GetBool(name)
	if err != nil {
		return false, fmt.Errorf("internal error reading --%s flag: %w", name, err)
	}

	return v, nil
}

// FlagString reads a String flag that has no config-file fallback. Wraps the cobra parse error
// with the flag name so callers can surface it as a real ExitError.
func FlagString(cmd *cobra.Command, name string) (string, error) {
	v, err := cmd.Flags().GetString(name)
	if err != nil {
		return "", fmt.Errorf("internal error reading --%s flag: %w", name, err)
	}

	return v, nil
}

// FlagStringArray reads a StringArray flag that has no config-file fallback. Wraps the cobra
// parse error with the flag name so callers can surface it as a real ExitError.
func FlagStringArray(cmd *cobra.Command, name string) ([]string, error) {
	v, err := cmd.Flags().GetStringArray(name)
	if err != nil {
		return nil, fmt.Errorf("internal error reading --%s flag: %w", name, err)
	}

	return v, nil
}

// FlagBoolOrDefault prefers an explicit --flag over cfgValue so the config default is honored silently when the user did not pass the flag.
// Returns the cobra parse error wrapped with the flag name so callers can surface it as a real ExitError.
func FlagBoolOrDefault(cmd *cobra.Command, name string, cfgValue bool) (bool, error) {
	if cmd.Flags().Changed(name) {
		v, err := cmd.Flags().GetBool(name)
		if err != nil {
			return false, fmt.Errorf("internal error reading --%s flag: %w", name, err)
		}

		return v, nil
	}

	return cfgValue, nil
}

// FlagStringSliceOrDefault returns the flag value when explicitly set (treating empty `--flag ""` as not passed), otherwise cfgValue.
// Returns the cobra parse error wrapped with the flag name so callers can surface it as a real ExitError.
func FlagStringSliceOrDefault(cmd *cobra.Command, name string, cfgValue []string) ([]string, error) {
	if cmd.Flags().Changed(name) {
		v, err := cmd.Flags().GetStringSlice(name)
		if err != nil {
			return nil, fmt.Errorf("internal error reading --%s flag: %w", name, err)
		}

		if len(v) > 0 {
			return v, nil
		}
	}

	return cfgValue, nil
}

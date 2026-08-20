package cmd

import (
	"github.com/spf13/cobra"
)

// addOptBoolFlag registers a Bool flag where `--name` (no value) means true.
// Used for opt-in feature flags like `--flat-paths`.
func addOptBoolFlag(cmd *cobra.Command, name string, defaultVal bool, usage string) {
	cmd.Flags().Bool(name, defaultVal, usage)
	cmd.Flags().Lookup(name).NoOptDefVal = "true"
}

// flagBoolOrDefault returns cfgValue unless the flag was explicitly set.
func flagBoolOrDefault(cmd *cobra.Command, name string, cfgValue bool) bool {
	if cmd.Flags().Changed(name) {
		v, _ := cmd.Flags().GetBool(name)
		return v
	}

	return cfgValue
}

// flagStringSliceOrDefault returns cfgValue unless the flag was explicitly set.
// Empty `--flag ""` is treated as "not passed" so we fall back to cfgValue.
func flagStringSliceOrDefault(cmd *cobra.Command, name string, cfgValue []string) []string {
	if cmd.Flags().Changed(name) {
		v, _ := cmd.Flags().GetStringSlice(name)
		if len(v) > 0 {
			return v
		}
	}

	return cfgValue
}

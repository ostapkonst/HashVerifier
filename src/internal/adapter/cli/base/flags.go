package base

import "github.com/spf13/cobra"

// AddOptBoolFlag registers a Bool flag that treats `--name` (no value) as true via NoOptDefVal, used by opt-in feature flags like --flat-paths.
func AddOptBoolFlag(cmd *cobra.Command, name string, defaultVal bool, usage string) {
	cmd.Flags().Bool(name, defaultVal, usage)
	cmd.Flags().Lookup(name).NoOptDefVal = "true"
}

// FlagBoolOrDefault returns the flag value when explicitly set by the user, otherwise cfgValue.
func FlagBoolOrDefault(cmd *cobra.Command, name string, cfgValue bool) bool {
	if cmd.Flags().Changed(name) {
		v, _ := cmd.Flags().GetBool(name)
		return v
	}

	return cfgValue
}

// FlagStringSliceOrDefault returns the flag value when explicitly set (treating empty `--flag ""` as not passed), otherwise cfgValue.
func FlagStringSliceOrDefault(cmd *cobra.Command, name string, cfgValue []string) []string {
	if cmd.Flags().Changed(name) {
		v, _ := cmd.Flags().GetStringSlice(name)
		if len(v) > 0 {
			return v
		}
	}

	return cfgValue
}

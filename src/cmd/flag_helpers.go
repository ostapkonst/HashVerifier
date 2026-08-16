package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func flagBoolOrDefault(cmd *cobra.Command, name string, cfgValue bool) bool {
	if cmd.Flags().Changed(name) {
		v, _ := cmd.Flags().GetBool(name)
		return v
	}

	return cfgValue
}

func flagStringSliceOrDefault(cmd *cobra.Command, name string, cfgValue []string) []string {
	if cmd.Flags().Changed(name) {
		v, _ := cmd.Flags().GetStringSlice(name)
		return v
	}

	return cfgValue
}

func normalizeAlgorithm(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(s, ".") {
		s = "." + s
	}

	return s
}

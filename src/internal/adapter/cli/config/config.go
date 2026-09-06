// Package config implements the `hashverifier config` subtree: show | edit | reset.
package config

import (
	"github.com/spf13/cobra"
)

// NewCmd assembles the config subcommand tree (show | edit | reset) for the root command.
// RunE prints help for a bare "config"; it is also what makes NoArgs reject unknown subcommands like
// "config bogus", because cobra returns help and exit code 0 before validating args on a non-runnable command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit HashVerifier settings",
		Long:  "View and edit HashVerifier configuration settings.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newShowCmd(), newEditCmd(), newResetCmd())

	return cmd
}

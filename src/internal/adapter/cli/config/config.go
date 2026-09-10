// Package config implements the `hashverifier config` subtree: show | edit | reset.
package config

import (
	"github.com/spf13/cobra"
)

// NewCmd assembles the config subcommand tree (show | edit | reset) for the root command.
// RunE prints help for a bare "config"; cobra's subcommand dispatcher rejects unknown subcommands
// like "config bogus" before RunE runs, exiting 1 with the "unknown command" error.
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

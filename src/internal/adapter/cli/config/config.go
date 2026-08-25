// Package config implements the `hashverifier config` subtree: show | edit | reset.
package config

import (
	"github.com/spf13/cobra"
)

// NewCmd assembles the config subcommand tree (show | edit | reset) for the root command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit HashVerifier settings",
		Long:  "View and edit HashVerifier configuration settings.",
		RunE:  runConfigShow,
	}

	cmd.AddCommand(newShowCmd(), newEditCmd(), newResetCmd())

	return cmd
}

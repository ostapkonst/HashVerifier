// Package cli is the CLI entry point that wires every subcommand and dispatches to the GUI when no subcommand is given.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/config"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/generate"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/hash"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/verify"
	guiapp "github.com/ostapkonst/HashVerifier/internal/adapter/gui/app"
	"github.com/ostapkonst/HashVerifier/internal/appmeta"
)

// Execute is the only CLI entry point the main package calls; the rest of the command tree stays package-private.
func Execute() error {
	return rootCmd.Execute()
}

var versionFlag bool

var rootCmd = &cobra.Command{
	Use:           "hashverifier",
	Short:         "Cross-platform checksum generation and verification tool",
	SilenceUsage:  true,
	SilenceErrors: true,
	Args:          cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
			fmt.Printf("%s %s\n", appmeta.Name, appmeta.Version)
			os.Exit(0)
		}

		if len(args) == 0 {
			return guiapp.Run("")
		}

		return guiapp.Run(args[0])
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print the version number")
	rootCmd.PersistentFlags().Bool("no-config", false,
		"Skip loading and saving settings (also via HASHVERIFIER_NO_CONFIG=1)")

	rootCmd.AddCommand(
		generate.NewCmd(),
		verify.NewCmd(),
		hash.NewCmd(),
		config.NewCmd(),
	)
}

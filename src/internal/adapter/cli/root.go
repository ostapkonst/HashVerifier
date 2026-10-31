// Package cli is the entry point: GUI for 0–1 command-line args (any string is a literal
// path), CLI for 2+ args (first non-flag arg is the command name).
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/config"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/discover"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/generate"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/hash"
	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/verify"
	guiapp "github.com/ostapkonst/HashVerifier/internal/adapter/gui/app"
	"github.com/ostapkonst/HashVerifier/internal/platform/env"
)

// Execute is the only entry point the main package calls.
// With 0–1 command-line args (any string is a literal path) it opens the GUI;
// with 2+ args the first non-flag arg is dispatched as a CLI command name.
func Execute() error {
	args := os.Args[1:]

	if len(args) <= 1 {
		return runGUI(args)
	}

	return runCLI(args)
}

func runGUI(args []string) error {
	path := ""
	if len(args) == 1 {
		path = args[0]
	}

	return guiapp.Run(path, env.NoConfig())
}

func runCLI(args []string) error {
	cmd := newCLIRoot()
	cmd.SetArgs(args)

	return cmd.Execute()
}

func newCLIRoot() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "hashverifier",
		Short:         "Cross-platform checksum generation and verification tool",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return nil
			}

			return fmt.Errorf("unknown command: %s", strings.Join(args, " "))
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	base.AddOptBoolPersistentFlag(cmd, "no-config", false,
		"Skip loading and saving settings (also via HASHVERIFIER_NO_CONFIG=1)")

	cmd.AddCommand(
		generate.NewCmd(),
		verify.NewCmd(),
		hash.NewCmd(),
		discover.NewCmd(),
		config.NewCmd(),
	)

	return cmd
}

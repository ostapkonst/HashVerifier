package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/gui"
	"github.com/ostapkonst/HashVerifier/internal/header"
)

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
			fmt.Printf("%s %s\n", header.Name, header.Version)
			os.Exit(0)
		}

		if len(args) == 0 {
			return gui.Run("")
		}

		return gui.Run(args[0])
	},
}

func init() {
	rootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print the version number")

	rootCmd.AddCommand(
		newGenerateCmd(),
		newVerifyCmd(),
		newHashCmd(),
		newConfigCmd(),
	)
}

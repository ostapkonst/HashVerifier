package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
)

func newResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset settings to defaults",
		Long:  "Reset all settings to their default values.",
		RunE:  runConfigReset,
	}

	cmd.Flags().Bool("yes", false, "Skip confirmation prompt")

	return cmd
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	if base.LoadNoConfig(cmd) {
		return base.ReportError(
			"config reset failed.",
			"not available in --no-config mode.",
			"drop the --no-config flag, or use --no-config only with generate/hash/verify.",
			78, nil,
		)
	}

	skipConfirm, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return base.ReportError(
			"failed to read --yes flag.",
			err.Error(),
			"this should not happen — please report a bug.",
			1, err,
		)
	}

	if !skipConfirm {
		fmt.Println("This will reset all settings to their default values.")
		fmt.Print("Are you sure? (y/N): ")

		var response string
		fmt.Scanln(&response) //nolint:errcheck
		fmt.Println()

		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Reset canceled.")
			return nil
		}
	}

	if err := settings.Reset(); err != nil {
		return base.ReportError(
			"failed to reset settings.",
			err.Error(),
			"check filesystem permissions and disk space, then try again.",
			1, err,
		)
	}

	fmt.Println("Settings have been reset to default values.")

	return nil
}

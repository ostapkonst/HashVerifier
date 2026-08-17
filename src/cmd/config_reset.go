package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/settings"
)

func newConfigResetCmd() *cobra.Command {
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
	if loadNoConfig(cmd) {
		return fmt.Errorf("config reset is not available in --no-config mode")
	}

	skipConfirm, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("internal error reading --yes flag: %w", err)
	}

	if !skipConfirm {
		fmt.Println("This will reset all settings to their default values.")
		fmt.Print("Are you sure? (y/N): ")

		var response string
		fmt.Scanln(&response) //nolint:errcheck

		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	if err := settings.Reset(); err != nil {
		return fmt.Errorf("failed to reset settings: %w", err)
	}

	fmt.Println("Settings have been reset to default values.")

	return nil
}

package cmd

import (
	"fmt"
	"os"
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
		fmt.Fprintf(os.Stderr, "Error: config reset is not available in --no-config mode.\n")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: drop the --no-config flag, or use --no-config only with generate/hash/verify.\n")

		err := fmt.Errorf("config reset is not available in --no-config mode")

		return &ExitError{Code: 78, Err: err, Silent: true}
	}

	skipConfirm, err := cmd.Flags().GetBool("yes")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read --yes flag.\n")
		fmt.Fprintf(os.Stderr, "  Reason: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: this should not happen — please report a bug.\n")

		err = fmt.Errorf("internal error reading --yes flag: %w", err)

		return &ExitError{Code: 1, Err: err, Silent: true}
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
		fmt.Fprintf(os.Stderr, "Error: failed to reset settings.\n")
		fmt.Fprintf(os.Stderr, "  Reason: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: check filesystem permissions and disk space, then try again.\n")

		err = fmt.Errorf("failed to reset settings: %w", err)

		return &ExitError{Code: 1, Err: err, Silent: true}
	}

	fmt.Println("Settings have been reset to default values.")

	return nil
}

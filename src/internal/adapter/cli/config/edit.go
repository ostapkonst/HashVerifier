package config

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
	"github.com/ostapkonst/HashVerifier/internal/platform/editor"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit settings file",
		Long:  "Open the settings file in your default text editor for manual editing.",
		RunE:  runConfigEdit,
	}
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	if base.LoadNoConfig(cmd) {
		err := fmt.Errorf("config edit is not available in --no-config mode")

		return base.ReportError(
			"config edit is not available in --no-config mode.",
			"drop the --no-config flag, or use --no-config only with generate/hash/verify.",
			78, err,
		)
	}

	path, err := settings.GetSettingsPath()
	if err != nil {
		return base.ReportError(
			"cannot determine settings file path.",
			"this should not happen — please report a bug.",
			1, fmt.Errorf("failed to get settings path: %w", err),
		)
	}

	editor := editor.Default()
	if editor == "" {
		err := fmt.Errorf("no text editor found; please set $EDITOR or $VISUAL environment variable")

		return base.ReportError(
			"no text editor found.",
			"set $EDITOR or $VISUAL environment variable to a text editor binary.",
			78, err,
		)
	}

	editCmd := exec.CommandContext(cmd.Context(), editor, path)

	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr

	fmt.Printf("Editing settings file: %s\n", path)
	fmt.Printf("Using editor: %s\n", editor)
	fmt.Println()

	if err := editCmd.Run(); err != nil {
		return base.ReportError(
			fmt.Sprintf("failed to run editor %s.", editor),
			"this should not happen — please report a bug.",
			1, fmt.Errorf("failed to run editor: %w", err),
		)
	}

	edited, err := base.LoadForConfig(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Path: %s\n", path)

		return base.ReportError(
			"post-edit settings file is corrupt.",
			"edit and save again, or run 'hashverifier config reset --yes' to restore defaults.",
			78, err,
		)
	}

	fmt.Println("Settings saved successfully.")
	printRepairs(edited.LoadWarnings())

	return nil
}

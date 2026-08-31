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
	noConfig, err := base.LoadNoConfig(cmd)
	if err != nil {
		return base.ReportError(
			"failed to read --no-config flag.",
			err.Error(),
			"this should not happen — please report a bug.",
			1, err,
		)
	}

	if noConfig {
		return base.ReportError(
			"config edit failed.",
			"not available in --no-config mode.",
			"drop the --no-config flag, or use --no-config only with generate/hash/verify.",
			78, nil,
		)
	}

	path, err := settings.GetSettingsPath()
	if err != nil {
		return base.ReportError(
			"cannot determine settings file path.",
			err.Error(),
			"this should not happen — please report a bug.",
			1, err,
		)
	}

	editor := editor.Default()
	if editor == "" {
		err := fmt.Errorf("no text editor found; please set $EDITOR or $VISUAL environment variable")

		return base.ReportError(
			"no text editor found.",
			err.Error(),
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
			err.Error(),
			"this should not happen — please report a bug.",
			1, err,
		)
	}

	edited, err := base.LoadForConfig(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Path: %s\n", path)

		return base.ReportError(
			"post-edit settings file is corrupt.",
			err.Error(),
			"edit and save again, or run 'hashverifier config reset --yes' to restore defaults.",
			78, err,
		)
	}

	fmt.Println("Settings saved successfully.")
	printRepairs(edited.LoadWarnings())

	return nil
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/settings"
)

func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Edit settings file",
		Long:  "Open the settings file in your default text editor for manual editing.",
		RunE:  runConfigEdit,
	}
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	if loadNoConfig(cmd) {
		return fmt.Errorf("config edit is not available in --no-config mode")
	}

	path, err := settings.GetSettingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
	}

	editor := defaultEditor()
	if editor == "" {
		return fmt.Errorf("no text editor found; please set $EDITOR or $VISUAL environment variable")
	}

	editCmd := exec.CommandContext(cmd.Context(), editor, path)

	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr

	fmt.Printf("Editing settings file: %s\n", path)
	fmt.Printf("Using editor: %s\n\n", editor)

	if err := editCmd.Run(); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	if edited, err := settings.Load(loadNoConfig(cmd)); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Settings file may be invalid: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please check the file and try again.\n")
	} else {
		logLoadWarnings(edited)
		fmt.Println("Settings saved successfully.")
	}

	return nil
}

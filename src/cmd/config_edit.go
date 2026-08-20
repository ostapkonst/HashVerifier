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
		fmt.Fprintf(os.Stderr, "Error: config edit is not available in --no-config mode.\n")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: drop the --no-config flag, or use --no-config only with generate/hash/verify.\n")

		err := fmt.Errorf("config edit is not available in --no-config mode")

		return &ExitError{Code: 78, Err: err, Silent: true}
	}

	path, err := settings.GetSettingsPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine settings file path.\n")
		fmt.Fprintf(os.Stderr, "  Reason: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: this should not happen — please report a bug.\n")

		err = fmt.Errorf("failed to get settings path: %w", err)

		return &ExitError{Code: 2, Err: err, Silent: true}
	}

	editor := defaultEditor()
	if editor == "" {
		fmt.Fprintf(os.Stderr, "Error: no text editor found.\n")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: set $EDITOR or $VISUAL environment variable to a text editor binary.\n")

		err := fmt.Errorf("no text editor found; please set $EDITOR or $VISUAL environment variable")

		return &ExitError{Code: 78, Err: err, Silent: true}
	}

	editCmd := exec.CommandContext(cmd.Context(), editor, path)

	editCmd.Stdin = os.Stdin
	editCmd.Stdout = os.Stdout
	editCmd.Stderr = os.Stderr

	fmt.Printf("Editing settings file: %s\n", path)
	fmt.Printf("Using editor: %s\n", editor)
	fmt.Println()

	if err := editCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to run editor %s.\n", editor)
		fmt.Fprintf(os.Stderr, "  Reason: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: this should not happen — please report a bug.\n")

		err = fmt.Errorf("failed to run editor: %w", err)

		return &ExitError{Code: 2, Err: err, Silent: true}
	}

	edited, err := loadForConfig(cmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: post-edit settings file is corrupt.\n")
		fmt.Fprintf(os.Stderr, "  Path:   %s\n", path)
		fmt.Fprintf(os.Stderr, "  Reason: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "Hint: edit and save again, or run 'hashverifier config reset --yes' to restore defaults.\n")

		return &ExitError{Code: 78, Err: err, Silent: true}
	}

	fmt.Println("Settings saved successfully.")
	printRepairs(edited.LoadWarnings())

	return nil
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/settings"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and edit HashVerifier settings",
	Long:  "View and edit HashVerifier configuration settings.",
	RunE:  runConfigShow,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current settings",
	Long:  "Display all current HashVerifier settings with descriptions.",
	RunE:  runConfigShow,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit settings file",
	Long:  "Open the settings file in your default text editor for manual editing.",
	RunE:  runConfigEdit,
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset settings to defaults",
	Long:  "Reset all settings to their default values.",
	RunE:  runConfigReset,
}

func init() {
	configResetCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := settings.Load()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	path, err := settings.GetSettingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
	}

	defaults := settings.DefaultSettings()
	settingsInfo := settings.GetAllSettingsInfo(cfg, defaults)

	fmt.Printf("Config file: %s\n\n", path)

	for _, section := range settingsInfo {
		fmt.Printf("%s Settings:\n", section.Name)
		fmt.Println(strings.Repeat("-", 80))

		for _, info := range section.Settings {
			printSetting(info.Name, info.Value, info.Description, info.Default)
		}
	}

	return nil
}

func printSetting(name, value, description, defaultValue string) {
	fmt.Printf("  Parameter:   %s\n", name)
	fmt.Printf("  Value:       %s\n", value)
	fmt.Printf("  Default:     %s\n", defaultValue)
	fmt.Printf("  Description: %s\n", description)
	fmt.Println()
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	path, err := settings.GetSettingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
	}

	editor := getDefaultEditor()
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

	if _, err := settings.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Settings file may be invalid: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please check the file and try again.\n")
	} else {
		fmt.Println("Settings saved successfully.")
	}

	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
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

func getDefaultEditor() string {
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// тут можно убрать windows case потому, что для Windows пользователей мы собираем только GUI
	switch runtime.GOOS {
	case "windows":
		defaultEditors := []string{"notepad.exe", "code", "notepad++.exe"}
		for _, ed := range defaultEditors {
			if path, err := exec.LookPath(ed); err == nil {
				return path
			}
		}

		return "notepad.exe"

	case "darwin":
		defaultEditors := []string{"vim", "nano", "vi", "open -t"}
		for _, ed := range defaultEditors {
			if path, err := exec.LookPath(ed); err == nil {
				return path
			}
		}

		return "vim"

	default:
		defaultEditors := []string{"vim", "nano", "vi"}
		for _, ed := range defaultEditors {
			if path, err := exec.LookPath(ed); err == nil {
				return path
			}
		}

		return "vi"
	}
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configResetCmd)

	rootCmd.AddCommand(configCmd)
}

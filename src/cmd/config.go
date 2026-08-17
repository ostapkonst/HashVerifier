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

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and edit HashVerifier settings",
		Long:  "View and edit HashVerifier configuration settings.",
		RunE:  runConfigShow,
	}

	cmd.AddCommand(newConfigShowCmd(), newConfigEditCmd(), newConfigResetCmd())

	return cmd
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := settings.Load(loadNoConfig(cmd))
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

	warnings := cfg.LoadWarnings()
	if len(warnings) > 0 {
		fmt.Println()
		fmt.Println(strings.Repeat("=", 80))
		fmt.Printf("Repairs applied (%d):\n", len(warnings))
		fmt.Println(strings.Repeat("=", 80))

		for _, w := range warnings {
			fmt.Printf("  %s: %q -> %q\n", w.Field, w.Value, w.Default)
		}
	}

	return nil
}

func defaultEditor() string {
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

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
		defaultEditors := []string{"vim", "nano", "vi"}
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

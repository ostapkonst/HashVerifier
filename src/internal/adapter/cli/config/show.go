package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ostapkonst/HashVerifier/internal/adapter/cli/base"
	settings "github.com/ostapkonst/HashVerifier/internal/driver/yamlconfig"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display current settings",
		Long:  "Display all current HashVerifier settings with descriptions.",
		RunE:  runConfigShow,
	}
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := base.LoadForConfig(cmd)
	if err != nil {
		path, _ := settings.GetSettingsPath()
		fmt.Fprintf(os.Stderr, "Path: %s\n", path)

		return base.ReportError(
			"settings file is corrupt.",
			"run 'hashverifier config reset --yes' to restore defaults.",
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

	defaults := settings.DefaultSettings()
	settingsInfo := settings.GetAllSettingsInfo(cfg, defaults)

	fmt.Printf("Config file: %s\n", path)
	fmt.Println()

	for i, section := range settingsInfo {
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("%s Settings:\n", section.Name)
		fmt.Println(strings.Repeat("-", 80))

		for j, info := range section.Settings {
			if j > 0 {
				fmt.Println()
			}

			printSetting(info.Name, info.Value, info.Description, info.Default)
		}
	}

	warnings := cfg.LoadWarnings()
	printRepairs(warnings)

	return nil
}

func printSetting(name, value, description, defaultValue string) {
	fmt.Printf("  Parameter:   %s\n", name)
	fmt.Printf("  Value:       %s\n", value)
	fmt.Printf("  Default:     %s\n", defaultValue)
	fmt.Printf("  Description: %s\n", description)
}

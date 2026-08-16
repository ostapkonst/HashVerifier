package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display current settings",
		Long:  "Display all current HashVerifier settings with descriptions.",
		RunE:  runConfigShow,
	}
}

func printSetting(name, value, description, defaultValue string) {
	fmt.Printf("  Parameter:   %s\n", name)
	fmt.Printf("  Value:       %s\n", value)
	fmt.Printf("  Default:     %s\n", defaultValue)
	fmt.Printf("  Description: %s\n", description)
	fmt.Println()
}

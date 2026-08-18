package cmd

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
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

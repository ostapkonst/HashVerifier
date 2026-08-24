// Package editor selects the user's preferred command-line text editor for editing the settings file.
package editor

import (
	"os"
	"os/exec"
	"runtime"
)

// Default returns the user's preferred text editor: $VISUAL → $EDITOR → known OS binaries (notepad.exe/code/notepad++.exe on Windows; vim/nano/vi on other platforms), with a final fallback.
func Default() string {
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

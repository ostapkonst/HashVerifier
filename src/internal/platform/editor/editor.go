// Package editor selects the user's preferred command-line text editor for editing the settings file.
package editor

import (
	"os"
	"os/exec"
	"runtime"
)

// Default returns the user's preferred text editor, honoring the standard precedence $VISUAL → $EDITOR → known OS binaries.
// Returns "" when no editor is configured or installed, so callers can surface a clean "no editor found" error instead of attempting to exec a missing binary.
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

	default:
		defaultEditors := []string{"vim", "nano", "vi"}
		for _, ed := range defaultEditors {
			if path, err := exec.LookPath(ed); err == nil {
				return path
			}
		}
	}

	return ""
}

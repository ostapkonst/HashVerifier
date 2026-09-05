// Package editor selects the user's preferred command-line text editor for editing the settings file.
package editor

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
)

// ErrNoEditor is returned when no $VISUAL/$EDITOR is set and no default editor is installed on the system.
var ErrNoEditor = errors.New("no text editor found; please set $EDITOR or $VISUAL environment variable")

// Default returns the user's preferred text editor, honoring the standard precedence $VISUAL → $EDITOR → known OS binaries.
// Returns "" when no editor is configured or installed, so callers can surface ErrNoEditor instead of attempting to exec a missing binary.
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

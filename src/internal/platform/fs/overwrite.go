// Package fs guards filesystem writes against accidental overwrites.
package fs

import (
	"errors"
	"fmt"
	"os"
)

// ErrRefuseOverwrite is returned when an output path exists and force is false.
var ErrRefuseOverwrite = errors.New("refusing to overwrite existing file")

// ShouldOverwrite returns nil if path does not exist or force is true, ErrRefuseOverwrite if it exists and force is false, or a wrapped error for other failures (stat, directory).
func ShouldOverwrite(path string, force bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("stat output file: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("output path is a directory: %s", path)
	}

	if !force {
		return ErrRefuseOverwrite
	}

	return nil
}

// Package fs guards filesystem writes against accidental overwrites.
package fs

import (
	"errors"
	"fmt"
	"os"
)

// ErrRefuseOverwrite is returned when an output path exists and force is false.
var ErrRefuseOverwrite = errors.New("refusing to overwrite existing file")

// ShouldOverwrite refuses to overwrite unless force=true so accidental data loss is avoided.
func ShouldOverwrite(path string, force bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
		return err
	}

	if info.IsDir() {
		return fmt.Errorf("output path is a directory: %s", path)
	}

	if !force {
		return ErrRefuseOverwrite
	}

	return nil
}

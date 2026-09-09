// Package fs guards filesystem writes against accidental overwrites and unsafe output targets.
package fs

import (
	"errors"
	"fmt"
	"os"
)

// ErrRefuseOverwrite is returned when an output path exists and force is false.
var ErrRefuseOverwrite = errors.New("refusing to overwrite existing file")

// ErrInvalidOutputPath is returned when the output path exists but is not safe to
// write to: a directory, FIFO, socket, device node, or a symlink (whose target we
// refuse to follow so callers cannot accidentally overwrite unrelated files or
// silently drop bytes into /dev/null-like targets).
var ErrInvalidOutputPath = errors.New("output path is not a regular file")

// IsSymlink reports whether path is a symbolic link. A missing path is not an
// error and reports false; any other stat failure is returned.
func IsSymlink(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return info.Mode()&os.ModeSymlink != 0, nil
}

// ShouldOverwrite decides whether path can be used as a writable output.
//
// It refuses when the existing path is:
//   - a directory,
//   - a symlink to anything (regular or non-regular) — writing through a symlink
//     either silently hits an unsafe target (a regular file outside the source tree
//     would be silently overwritten) or hits /dev/null-like targets; callers can
//     remove the symlink first if a real file is wanted,
//   - a non-regular inode (FIFO, socket, device node),
//
// If the path does not exist, the function returns nil so callers can create the
// file fresh. With force=false, an existing regular file returns ErrRefuseOverwrite;
// with force=true, the overwrite is allowed.
func ShouldOverwrite(path string, force bool) error {
	if isSymlink, err := IsSymlink(path); err != nil {
		// fs.PathError already embeds the op ("lstat") and path; another prefix would double them.
		return err
	} else if isSymlink {
		return fmt.Errorf("%w: %s", ErrInvalidOutputPath, path)
	}

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

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrInvalidOutputPath, path)
	}

	if !force {
		return ErrRefuseOverwrite
	}

	return nil
}

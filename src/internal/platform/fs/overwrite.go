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
// write to: a FIFO, socket, device node, broken symlink, or a symlink to any of
// those. A symlink whose target is a regular file is allowed — the user explicitly
// asked for that target by creating the link.
var ErrInvalidOutputPath = errors.New("output path is not a regular file")

// ErrOutputIsDirectory is returned when the output path is a directory or a
// symlink whose target is a directory.
var ErrOutputIsDirectory = errors.New("output path is a directory")

// ShouldOverwrite decides whether path can be used as a writable output.
//
// It refuses when the existing path (or the target of a symlink) is:
//   - a directory,
//   - a non-regular inode (FIFO, socket, device node) — including via symlink,
//   - a symlink whose target is missing or inaccessible (treat as unsafe).
//
// A symlink whose target is a regular file IS allowed, per docs (USAGE.md §Generate Checksums):
// "The output path must also be a regular file or a symlink whose target is a regular file".
//
// If the path does not exist, the function returns nil so callers can create the
// file fresh. With force=false, an existing regular file returns ErrRefuseOverwrite;
// with force=true, the overwrite is allowed.
func ShouldOverwrite(path string, force bool) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Path genuinely doesn't exist OR is a broken symlink (Stat follows
			// the link and surfaces ENOENT for the missing target). Lstat again
			// to distinguish: only a broken symlink is treated as unsafe.
			if linfo, lerr := os.Lstat(path); lerr == nil && linfo.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("%w: %s", ErrInvalidOutputPath, path)
			}

			return nil
		}

		// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
		return err
	}

	if info.IsDir() {
		return fmt.Errorf("%w: %s", ErrOutputIsDirectory, path)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrInvalidOutputPath, path)
	}

	if !force {
		return fmt.Errorf("%w: %s", ErrRefuseOverwrite, path)
	}

	return nil
}

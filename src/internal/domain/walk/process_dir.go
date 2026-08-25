// Package walk enumerates files under a directory tree using godirwalk and provides path-validation helpers.
package walk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karrick/godirwalk"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
)

// IsPathValidationError reports whether err is one of the path-validation sentinels from hashfn.
func IsPathValidationError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, hashfn.ErrPathContainsInvalidSeparator) ||
		errors.Is(err, hashfn.ErrPathContainsNewline) ||
		errors.Is(err, hashfn.ErrPathContainsCarriageReturn) ||
		errors.Is(err, hashfn.ErrCRC32PathStartsWithSemicolon) ||
		errors.Is(err, hashfn.ErrCRC32PathEndWithSpace)
}

// WalkDir lists files under path. followSymbolicLinks controls recursion through symlinks; sortPaths orders results.
func WalkDir(ctx context.Context, path string, followSymbolicLinks, sortPaths bool) ([]string, error) {
	var files []string

	err := godirwalk.Walk(path, &godirwalk.Options{
		FollowSymbolicLinks: followSymbolicLinks,
		Unsorted:            !sortPaths,

		Callback: func(path string, de *godirwalk.Dirent) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if b, _ := de.IsDirOrSymlinkToDir(); b {
				return nil
			}

			files = append(files, path)

			return nil
		},

		ErrorCallback: func(path string, err error) godirwalk.ErrorAction {
			select {
			case <-ctx.Done():
				return godirwalk.Halt
			default:
			}

			if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
				return godirwalk.SkipNode
			}

			return godirwalk.Halt
		},
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if err != nil {
		return nil, err
	}

	return files, nil
}

// GetPrefixForFilesInChecksum returns the path prefix to prepend to entries (sibling dir basename, or folder abs path).
func GetPrefixForFilesInChecksum(folder, file string) (string, error) {
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for folder: %w", err)
	}

	absFile, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for file: %w", err)
	}

	if hashfn.PathsEqual(filepath.Dir(absFolder), filepath.Dir(absFile)) {
		return filepath.Base(absFolder), nil
	}

	return absFolder, nil
}

// FormatLine renders one checksum-file line: path-first for CRC32/SFV, hash-first (`*path`) for *SUMS.
func FormatLine(relPath, hashStr string, algoType algorithm.Algorithm) string {
	switch algoType {
	case algorithm.CRC32:
		return fmt.Sprintf("%s %s", relPath, hashStr)
	default:
		return fmt.Sprintf("%s *%s", hashStr, relPath)
	}
}

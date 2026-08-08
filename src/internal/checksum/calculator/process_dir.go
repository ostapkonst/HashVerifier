package calculator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/karrick/godirwalk"

	"github.com/ostapkonst/HashVerifier/internal/checksum/algo"
)

func IsPathValidationError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, ErrPathContainsInvalidSeparator) ||
		errors.Is(err, ErrPathContainsNewline) ||
		errors.Is(err, ErrPathContainsCarriageReturn) ||
		errors.Is(err, ErrCRC32PathStartsWithSemicolon) ||
		errors.Is(err, ErrCRC32PathEndWithSpace)
}

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

func GetPrefixForFilesInChecksum(folder, file string) (string, error) {
	absFolder, err := filepath.Abs(folder)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for folder: %w", err)
	}

	absFile, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for file: %w", err)
	}

	if filepath.Dir(absFolder) == filepath.Dir(absFile) {
		return filepath.Base(absFolder), nil
	}

	return absFolder, nil
}

func FormatLine(relPath, hashStr string, algoType algo.Algorithm) string {
	switch algoType {
	case algo.CRC32:
		return fmt.Sprintf("%s %s", relPath, hashStr)
	default:
		return fmt.Sprintf("%s *%s", hashStr, relPath)
	}
}

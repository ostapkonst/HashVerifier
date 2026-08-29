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

// SkippedEntry pairs a path WalkDir had to skip with the underlying error (permission, not-exist, etc.).
type SkippedEntry struct {
	Path string
	Err  error
}

// WalkResult is what WalkDir returns: the files it could list and the skipped entries (permission/not-exist,
// non-regular types, broken-symlink targets); other walk errors halt and surface as err.
type WalkResult struct {
	Files   []string
	Skipped []SkippedEntry
}

// WalkDir lists files under path. followSymbolicLinks controls whether symlink entries participate: when true,
// file symlinks are hashed and directory symlinks are descended into; when false, symlink entries are excluded
// entirely. Broken symlinks are recorded in Skipped only when following. Other errors halt the walk.
func WalkDir(ctx context.Context, path string, followSymbolicLinks, sortPaths bool) (WalkResult, error) {
	var result WalkResult

	// godirwalk Lstats the root when followSymbolicLinks is false (walk.go:227-238), so a symlinked root
	// would be rejected as "cannot Walk non-directory". Resolve the root, walk it, then re-prefix the
	// user-given root segment back onto entries so checksum paths keep the name the user provided.
	root, err := filepath.EvalSymlinks(path)
	if err != nil {
		return WalkResult{}, fmt.Errorf("resolve root %s: %w", path, err)
	}

	reRootEntries := root != path

	err = godirwalk.Walk(root, &godirwalk.Options{
		FollowSymbolicLinks: followSymbolicLinks,
		Unsorted:            !sortPaths,

		Callback: func(path string, de *godirwalk.Dirent) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if de.IsSymlink() {
				return symlinkCallback(path, followSymbolicLinks, &result)
			}

			if b, _ := de.IsDirOrSymlinkToDir(); b {
				return nil
			}

			// Non-regular nodes (FIFOs, sockets, devices) are excluded: opening them for hashing can block
			// forever (a FIFO read waits for a writer), which freezes generation and defeats SIGINT/SIGTERM.
			if !de.IsRegular() {
				result.Skipped = append(result.Skipped, SkippedEntry{
					Path: path,
					Err:  fmt.Errorf("non-regular file type %s (not hashed)", de.ModeType()),
				})

				return nil
			}

			result.Files = append(result.Files, path)

			return nil
		},

		ErrorCallback: func(p string, err error) godirwalk.ErrorAction {
			select {
			case <-ctx.Done():
				return godirwalk.Halt
			default:
			}

			if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
				result.Skipped = append(result.Skipped, SkippedEntry{Path: p, Err: err})
				return godirwalk.SkipNode
			}

			return godirwalk.Halt
		},
	})

	select {
	case <-ctx.Done():
		return WalkResult{}, fmt.Errorf("walk %s: %w", path, ctx.Err())
	default:
	}

	if err != nil {
		return WalkResult{}, fmt.Errorf("walk %s: %w", path, err)
	}

	if reRootEntries {
		result.Files = reRootPaths(result.Files, root, path)
		result.Skipped = reRootSkipped(result.Skipped, root, path)
	}

	return result, nil
}

// reRootPaths replaces the resolved root segment with the user-given one so entries keep the provided root's name.
func reRootPaths(paths []string, resolvedRoot, originalRoot string) []string {
	for i, p := range paths {
		if rel, err := filepath.Rel(resolvedRoot, p); err == nil {
			paths[i] = filepath.Join(originalRoot, rel)
		}
	}

	return paths
}

// reRootSkipped applies the same root-name substitution to walk-skipped entries so warnings match checksum entry naming.
func reRootSkipped(skipped []SkippedEntry, resolvedRoot, originalRoot string) []SkippedEntry {
	for i := range skipped {
		if rel, err := filepath.Rel(resolvedRoot, skipped[i].Path); err == nil {
			skipped[i].Path = filepath.Join(originalRoot, rel)
		}
	}

	return skipped
}

// symlinkCallback classifies one symlink entry: skips the whole node when not following, records broken links as
// SkippedEntry, appends file-symlinks to Files when following, and lets godirwalk descend into directory symlinks.
// Returning godirwalk.SkipThis ends this node cleanly without involving the walk ErrorCallback (no double-report).
func symlinkCallback(path string, followSymbolicLinks bool, result *WalkResult) error {
	if !followSymbolicLinks {
		return godirwalk.SkipThis
	}

	info, err := os.Stat(path)
	if err != nil {
		result.Skipped = append(result.Skipped, SkippedEntry{
			Path: path,
			Err:  fmt.Errorf("symlink target: %w", err),
		})

		return godirwalk.SkipThis
	}

	if info.IsDir() {
		return nil
	}

	// Non-regular targets (FIFOs, sockets, devices) are excluded: opening them for hashing can block forever.
	if !info.Mode().IsRegular() {
		result.Skipped = append(result.Skipped, SkippedEntry{
			Path: path,
			Err:  fmt.Errorf("symlink to non-regular target %s (not hashed)", info.Mode().Type()),
		})

		return godirwalk.SkipThis
	}

	result.Files = append(result.Files, path)

	return nil
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

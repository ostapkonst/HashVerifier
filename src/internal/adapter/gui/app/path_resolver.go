package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// PathType classifies a filesystem path passed via CLI or drag-and-drop.
type PathType int

// PathType values: invalid (missing/empty), directory, or file.
const (
	PathTypeInvalid PathType = iota
	PathTypeDirectory
	PathTypeFile
)

// PathResolver normalizes a path string and reports its type.
type PathResolver struct{}

// NewPathResolver returns a zero-state PathResolver ready for Resolve.
func NewPathResolver() *PathResolver {
	return &PathResolver{}
}

// Resolve classifies path so the caller can route it to Generate, Verify, or Hash without re-doing the stat.
//
// An empty path means the caller has nothing to route — the GUI keeps the tab restored
// from settings. "." and ".." are relative references to the current and parent directory
// that the OS always resolves; we expand them to absolute paths so downstream code can
// show a meaningful entryDir label and GenChecksumFilename does not treat them as roots.
func (pr *PathResolver) Resolve(path string) (PathType, string, error) {
	if path == "" {
		return PathTypeInvalid, "", nil
	}

	cleanPath := filepath.Clean(path)
	if cleanPath == "." || cleanPath == ".." {
		cwd, err := os.Getwd()
		if err != nil {
			return PathTypeInvalid, "", fmt.Errorf("resolve current directory: %w", err)
		}

		if cleanPath == "." {
			return PathTypeDirectory, cwd, nil
		}

		return PathTypeDirectory, filepath.Dir(cwd), nil
	}

	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		return PathTypeInvalid, "", fmt.Errorf("failed to access path: %w", err)
	}

	if fileInfo.IsDir() {
		return PathTypeDirectory, cleanPath, nil
	}

	return PathTypeFile, cleanPath, nil
}

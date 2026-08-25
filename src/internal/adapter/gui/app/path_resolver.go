package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// Resolve returns the path's PathType, the cleaned absolute path, or a stat error.
func (pr *PathResolver) Resolve(path string) (PathType, string, error) {
	cleanPath := filepath.Clean(path)
	if cleanPath == "." {
		return PathTypeInvalid, "", nil
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

// URIToFilePath decodes a file:// URI into a local path, handling Windows drive-letter and UNC forms.
func URIToFilePath(uri string) (string, error) {
	uri = strings.TrimRight(strings.TrimSpace(uri), "\r\n")
	if uri == "" {
		return "", fmt.Errorf("empty URI")
	}

	parsedURL, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}

	if parsedURL.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme: %s", parsedURL.Scheme)
	}

	path := parsedURL.Path

	if runtime.GOOS == "windows" {
		switch {
		case parsedURL.Host != "":
			path = `\\` + parsedURL.Host + filepath.FromSlash(path)
		case len(path) > 2 && path[0] == '/' && path[2] == ':':
			path = filepath.FromSlash(path[1:])
		default:
			return "", fmt.Errorf("unsupported file URI on Windows: %s", uri)
		}
	}

	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("failed to unescape path: %w", err)
	}

	return decodedPath, nil
}

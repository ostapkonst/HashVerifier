package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type PathType int

const (
	PathTypeInvalid PathType = iota
	PathTypeDirectory
	PathTypeFile
)

type PathResolver struct{}

func NewPathResolver() *PathResolver {
	return &PathResolver{}
}

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
		if len(path) > 2 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}

		path = filepath.FromSlash(path)
	}

	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		return "", fmt.Errorf("failed to unescape path: %w", err)
	}

	return decodedPath, nil
}

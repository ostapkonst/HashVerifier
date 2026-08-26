// Package parser reads SFV and *SUMS checksum files into typed line records.
package parser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dimchansky/utfbom"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
)

var (
	hashFirstRe = regexp.MustCompile(`^\s*([a-fA-F0-9]+)\s+\*?(.+)$`)
	sfvRe       = regexp.MustCompile(`^(.+?)\s+([a-fA-F0-9]{8})\s*$`)
)

// CheckSumLine is one parsed entry: a rel-path paired with its hex digest.
type CheckSumLine struct {
	RelPath string
	Hash    string
}

// ParseCheckSum reads the checksum file and returns its entries. Honors ctx cancellation.
func ParseCheckSum(ctx context.Context, filename string, algoType algorithm.Algorithm) ([]CheckSumLine, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filename, err)
	}

	defer f.Close() //nolint:errcheck

	var lines []CheckSumLine

	scanner := bufio.NewScanner(utfbom.SkipOnly(f))
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		line := scanner.Text()

		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, ";") {
			continue
		}

		if algorithm.FormatFromAlgorithm(algoType) == algorithm.FormatHashFirst &&
			strings.HasPrefix(trimmedLine, "#") {
			continue
		}

		relPath, hash, err := parseLine(line, algoType)
		if err != nil {
			return nil, fmt.Errorf("parse line %q: %w", line, err)
		}

		lines = append(lines, CheckSumLine{
			RelPath: relPath,
			Hash:    hash,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", filename, err)
	}

	return lines, nil
}

func parseLine(line string, algoType algorithm.Algorithm) (relPath, expectedHash string, err error) {
	format := algorithm.FormatFromAlgorithm(algoType)

	switch format {
	case algorithm.FormatHashFirst:
		matches := hashFirstRe.FindStringSubmatch(line)
		if len(matches) != 3 {
			return "", "", fmt.Errorf("invalid hash-first line: %q", line)
		}

		expectedHash = matches[1]
		relPath = matches[2]
	case algorithm.FormatPathFirst:
		matches := sfvRe.FindStringSubmatch(line)
		if len(matches) != 3 {
			return "", "", fmt.Errorf("invalid SFV line: %q", line)
		}

		relPath = matches[1]
		expectedHash = matches[2]
	default:
		return "", "", errors.New("unknown format")
	}

	if !algorithm.IsValidHashLength(expectedHash, algoType) {
		return "", "", fmt.Errorf("invalid hash length %d for %s", len(expectedHash), algoType.String())
	}

	return fixPathSeparator(relPath), expectedHash, nil
}

func fixPathSeparator(p string) string {
	if p == "" {
		return ""
	}

	p = strings.ReplaceAll(p, `\`, "/")

	return filepath.Clean(p)
}

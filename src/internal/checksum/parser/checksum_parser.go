package parser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/dimchansky/utfbom"
	"github.com/ostapkonst/HashVerifier/internal/checksum/algo"
)

var (
	hashFirstRe = regexp.MustCompile(`^([a-fA-F0-9]+)\s+\*?(.+)$`)
	sfvRe       = regexp.MustCompile(`^(.+?)\s+([a-fA-F0-9]{8})$`)
)

type CheckSumLine struct {
	RelPath string
	Hash    string
}

func ParseCheckSum(ctx context.Context, filename string, algoType algo.Algorithm) ([]CheckSumLine, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	f, err := os.Open(filename)
	if err != nil {
		return nil, err
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
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		relPath, hash, err := parseLine(line, algoType)
		if err != nil {
			return nil, err
		}

		lines = append(lines, CheckSumLine{
			RelPath: relPath,
			Hash:    hash,
		})
	}

	return lines, scanner.Err()
}

func parseLine(line string, algoType algo.Algorithm) (relPath, expectedHash string, err error) {
	format := algo.FormatFromAlgorithm(algoType)

	switch format {
	case algo.FormatHashFirst:
		matches := hashFirstRe.FindStringSubmatch(line)
		if len(matches) != 3 {
			return "", "", fmt.Errorf("invalid hash-first line: %q", line)
		}

		expectedHash = matches[1]
		relPath = matches[2]
	case algo.FormatPathFirst:
		matches := sfvRe.FindStringSubmatch(line)
		if len(matches) != 3 {
			return "", "", fmt.Errorf("invalid SFV line: %q", line)
		}

		relPath = matches[1]
		expectedHash = matches[2]
	default:
		return "", "", errors.New("unknown format")
	}

	if !algo.IsValidHashLength(expectedHash, algoType) {
		return "", "", fmt.Errorf("invalid hash length %d for %s", len(expectedHash), algoType.String())
	}

	return fixPathSeparator(relPath), expectedHash, nil
}

func fixPathSeparator(path string) string {
	// стараемся сделать путь кросс-платформенным...
	return strings.ReplaceAll(path, "\\", string(os.PathSeparator))
}

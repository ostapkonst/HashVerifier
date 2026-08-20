package generator

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/internal/header"
	"github.com/ostapkonst/HashVerifier/utils/eof"
)

type GenerateConfig struct {
	InputDir            string
	OutputFile          string
	Algorithm           checksum.Algorithm
	DirPrefix           string
	FollowSymbolicLinks bool
	SortPaths           bool
	FlatPaths           bool
	ExcludeMatcher      *checksum.ExcludeMatcher
	OnFileHashed        func(result checksum.GenerateResult)
}

type GenerateResultStats struct {
	Stats checksum.GeneratorStats
}

func formatStatsFooter(stats checksum.GeneratorStats, runErr error) string {
	status := header.StatusSuccess

	switch {
	case errors.Is(runErr, context.Canceled):
		status = header.StatusCanceled
	case runErr != nil:
		status = header.StatusFailed
	case stats.WithErrors > 0 && stats.Skipped > 0:
		status = header.StatusCompletedWithErrorsSkipped
	case stats.WithErrors > 0:
		status = header.StatusCompletedWithErrors
	case stats.Skipped > 0:
		status = header.StatusCompletedWithSkipped
	}

	statsPending := stats.Pending()

	optionalNewLine := ""
	if stats.Processed+stats.WithErrors > 0 {
		optionalNewLine = eof.PlatformEOF
	}

	statistics := fmt.Sprintf(
		"%s"+
			"; Statistics:%s"+
			";   Status: %s%s",
		optionalNewLine,
		eof.PlatformEOF,
		status,
		eof.PlatformEOF,
	)

	if stats.Processed > 0 {
		statistics += fmt.Sprintf(
			";   Processed: %d%s",
			stats.Processed,
			eof.PlatformEOF,
		)
	}

	if stats.WithErrors > 0 {
		statistics += fmt.Sprintf(
			";   Failures: %d%s",
			stats.WithErrors,
			eof.PlatformEOF,
		)
	}

	if stats.Skipped > 0 {
		statistics += fmt.Sprintf(
			";   Skipped: %d%s",
			stats.Skipped,
			eof.PlatformEOF,
		)
	}

	if statsPending > 0 {
		statistics += fmt.Sprintf(
			";   Pending: %d%s",
			statsPending,
			eof.PlatformEOF,
		)
	}

	return statistics
}

func ValidateInputDir(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat input directory: %w", err)
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("input path is not a directory")
	}

	return nil
}

func ValidateOutputFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err == nil && fileInfo.IsDir() {
		return fmt.Errorf("output path is a directory: %s", path)
	}

	return nil
}

func GenerateChecksums(ctx context.Context, cfg GenerateConfig) (GenerateResultStats, error) {
	if err := ValidateInputDir(cfg.InputDir); err != nil {
		return GenerateResultStats{}, fmt.Errorf("invalid input dir: %w", err)
	}

	if err := ValidateOutputFile(cfg.OutputFile); err != nil {
		return GenerateResultStats{}, fmt.Errorf("invalid output file: %w", err)
	}

	if cfg.Algorithm == checksum.Unknown {
		return GenerateResultStats{}, fmt.Errorf("algorithm not specified")
	}

	f, err := os.Create(cfg.OutputFile)
	if err != nil {
		return GenerateResultStats{}, fmt.Errorf("failed to create checksum file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	bw := bufio.NewWriter(f)

	if _, err := bw.WriteString(header.GetChecksumHeader()); err != nil {
		return GenerateResultStats{}, fmt.Errorf("failed to write program header: %w", err)
	}

	generator := NewGeneratorWithExclusions(
		ctx, cfg.InputDir, cfg.OutputFile, cfg.Algorithm, cfg.DirPrefix,
		cfg.FollowSymbolicLinks, cfg.SortPaths,
		cfg.ExcludeMatcher,
	)
	generator.Start()

	var hasError error

	for res := range generator.Results() {
		if !checksum.IsPathValidationError(res.Err) && !checksum.IsExcludedError(res.Err) {
			line := checksum.FormatLine(res.RelPath, res.Hash, cfg.Algorithm)

			if _, err = bw.WriteString(line + eof.PlatformEOF); err != nil {
				hasError = fmt.Errorf("failed to write line: %w", err)
				break
			}
		}

		if cfg.OnFileHashed != nil {
			cfg.OnFileHashed(res)
		}
	}

	if err := generator.Wait(); err != nil && hasError == nil {
		hasError = fmt.Errorf("failed to generate checksums: %w", err)
	}

	if _, err := bw.WriteString(formatStatsFooter(generator.Stats(), hasError)); err != nil && hasError == nil {
		hasError = fmt.Errorf("failed to write stats footer: %w", err)
	}

	if err := bw.Flush(); err != nil && hasError == nil {
		hasError = fmt.Errorf("failed to flush buffer: %w", err)
	}

	return GenerateResultStats{Stats: generator.Stats()}, hasError
}

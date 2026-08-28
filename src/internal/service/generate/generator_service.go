package generate

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/appmeta"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	"github.com/ostapkonst/HashVerifier/internal/platform/eol"
)

// GenerateConfig is the shared input for GenerateChecksums and its streaming variant.
type GenerateConfig struct {
	InputDir            string
	OutputFile          string
	Algorithm           algorithm.Algorithm
	DirPrefix           string
	FollowSymbolicLinks bool
	SortPaths           bool
	FlatPaths           bool
	ExcludeMatcher      *exclude.Matcher
	OnFileHashed        func(result result.GenerateResult)
}

// GenerateResultStats is the forward-compatible return value of GenerateChecksums.
type GenerateResultStats struct {
	Stats result.GeneratorStats
}

func formatStatsFooter(stats result.GeneratorStats, runErr error) string {
	status := appmeta.StatusSuccess

	switch {
	case errors.Is(runErr, context.Canceled):
		status = appmeta.StatusCanceled
	case runErr != nil:
		status = appmeta.StatusFailed
	case stats.WithErrors > 0 && stats.Skipped > 0:
		status = appmeta.StatusCompletedWithErrorsSkipped
	case stats.WithErrors > 0:
		status = appmeta.StatusCompletedWithErrors
	case stats.Skipped > 0:
		status = appmeta.StatusCompletedWithSkipped
	}

	statsPending := stats.Pending()

	optionalNewLine := ""
	if stats.Processed+stats.WithErrors > 0 {
		optionalNewLine = eol.PlatformEOL
	}

	statistics := fmt.Sprintf(
		"%s"+
			"; Statistics:%s"+
			";   Status: %s%s",
		optionalNewLine,
		eol.PlatformEOL,
		status,
		eol.PlatformEOL,
	)

	if stats.Processed > 0 {
		statistics += fmt.Sprintf(
			";   Processed: %d%s",
			stats.Processed,
			eol.PlatformEOL,
		)
	}

	if stats.WithErrors > 0 {
		statistics += fmt.Sprintf(
			";   Failures: %d%s",
			stats.WithErrors,
			eol.PlatformEOL,
		)
	}

	if stats.Skipped > 0 {
		statistics += fmt.Sprintf(
			";   Skipped: %d%s",
			stats.Skipped,
			eol.PlatformEOL,
		)
	}

	if statsPending > 0 {
		statistics += fmt.Sprintf(
			";   Pending: %d%s",
			statsPending,
			eol.PlatformEOL,
		)
	}

	return statistics
}

// ValidateInputDir rejects paths that do not exist or are not directories, before the walk begins.
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

// ValidateOutputFile returns an error if path exists and is a directory; missing path is OK (will be created).
func ValidateOutputFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err == nil && fileInfo.IsDir() {
		return fmt.Errorf("output path is a directory: %s", path)
	}

	return nil
}

// GenerateChecksums is the blocking entry point; it runs the pipeline to completion before returning.
func GenerateChecksums(ctx context.Context, cfg GenerateConfig) (GenerateResultStats, error) {
	if err := ValidateInputDir(cfg.InputDir); err != nil {
		return GenerateResultStats{}, fmt.Errorf("invalid input dir: %w", err)
	}

	if err := ValidateOutputFile(cfg.OutputFile); err != nil {
		return GenerateResultStats{}, fmt.Errorf("invalid output file: %w", err)
	}

	if cfg.Algorithm == algorithm.Unknown {
		return GenerateResultStats{}, algorithm.ErrAlgorithmNotSpecified
	}

	f, err := os.Create(cfg.OutputFile)
	if err != nil {
		return GenerateResultStats{}, fmt.Errorf("failed to create checksum file: %w", err)
	}

	var hasError error

	bw := bufio.NewWriter(f)

	if _, err := bw.WriteString(appmeta.GetChecksumHeader()); err != nil {
		hasError = fmt.Errorf("failed to write program header: %w", err)
		if cerr := f.Close(); cerr != nil {
			hasError = errors.Join(hasError, fmt.Errorf("close checksum file: %w", cerr))
		}

		return GenerateResultStats{}, hasError
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	generator := NewGeneratorWithExclusions(
		ctx, cfg.InputDir, cfg.OutputFile, cfg.Algorithm, cfg.DirPrefix,
		cfg.FollowSymbolicLinks, cfg.SortPaths,
		cfg.ExcludeMatcher,
	)
	generator.Start()

	for res := range generator.Results() {
		if !walk.IsPathValidationError(res.Err) && !exclude.IsExcludedError(res.Err) {
			line := walk.FormatLine(res.RelPath, res.Hash, cfg.Algorithm)

			if _, err = bw.WriteString(line + eol.PlatformEOL); err != nil {
				hasError = fmt.Errorf("failed to write line: %w", err)

				break
			}
		}

		if cfg.OnFileHashed != nil {
			cfg.OnFileHashed(res)
		}

		generator.MarkWritten(res.Err)
	}

	cancel()

	if err := generator.Wait(); err != nil {
		hasError = errors.Join(hasError, fmt.Errorf("failed to generate checksums: %w", err))
	}

	if _, err := bw.WriteString(formatStatsFooter(generator.Stats(), hasError)); err != nil {
		hasError = errors.Join(hasError, fmt.Errorf("failed to write stats footer: %w", err))
	}

	if err := bw.Flush(); err != nil {
		hasError = errors.Join(hasError, fmt.Errorf("failed to flush buffer: %w", err))
	}

	if cerr := f.Close(); cerr != nil {
		hasError = errors.Join(hasError, fmt.Errorf("close checksum file: %w", cerr))
	}

	return GenerateResultStats{Stats: generator.Stats()}, hasError
}

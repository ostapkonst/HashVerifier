package generator

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
	"github.com/ostapkonst/HashVerifier/internal/header"
	"github.com/ostapkonst/HashVerifier/utils/eof"
)

const statsUpdateInterval = 50 * time.Millisecond

type GenerateStreamingResult struct {
	Result           checksum.GenerateResult
	Stats            checksum.GeneratorStats
	Err              error
	IsProgressUpdate bool
}

type GenerateStreamingConfig struct {
	InputDir            string
	OutputFile          string
	Algorithm           checksum.Algorithm
	DirPrefix           string
	FollowSymbolicLinks bool
	SortPaths           bool
	FlatPaths           bool
	ExcludeMatcher      *checksum.ExcludeMatcher
}

func GenerateChecksumsStreamingToFile(ctx context.Context, cfg GenerateStreamingConfig) (<-chan GenerateStreamingResult, error) {
	if err := ValidateInputDir(cfg.InputDir); err != nil {
		return nil, fmt.Errorf("invalid input dir: %w", err)
	}

	if err := ValidateOutputFile(cfg.OutputFile); err != nil {
		return nil, fmt.Errorf("invalid output file: %w", err)
	}

	if cfg.Algorithm == checksum.Unknown {
		return nil, fmt.Errorf("algorithm not specified")
	}

	f, err := os.Create(cfg.OutputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create checksum file: %w", err)
	}

	bw := bufio.NewWriter(f)
	if _, err := bw.WriteString(header.GetChecksumHeader()); err != nil {
		f.Close() //nolint:errcheck
		return nil, fmt.Errorf("failed to write program header: %w", err)
	}

	resultCh := make(chan GenerateStreamingResult, 1)

	go func() {
		defer f.Close() //nolint:errcheck
		defer close(resultCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		generator := NewGeneratorWithExclusions(
			ctx, cfg.InputDir, cfg.OutputFile, cfg.Algorithm, cfg.DirPrefix,
			cfg.FollowSymbolicLinks, cfg.SortPaths,
			cfg.ExcludeMatcher,
		)
		generator.Start()

		var hasError error

		done := make(chan struct{})

		go func() {
			defer close(done)

			ticker := time.NewTicker(statsUpdateInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					select {
					case resultCh <- GenerateStreamingResult{
						Stats:            generator.Stats(),
						IsProgressUpdate: true,
					}:
					default:
					}
				}
			}
		}()

		for res := range generator.Results() {
			if !checksum.IsPathValidationError(res.Err) && !checksum.IsExcludedError(res.Err) {
				line := checksum.FormatLine(res.RelPath, res.Hash, cfg.Algorithm)

				if _, err := bw.WriteString(line + eof.PlatformEOF); err != nil {
					hasError = fmt.Errorf("failed to write line: %w", err)
					break
				}
			}

			resultCh <- GenerateStreamingResult{
				Result: res,
				Stats:  generator.Stats(),
			}
		}

		cancel()

		if err := generator.Wait(); err != nil && hasError == nil {
			hasError = fmt.Errorf("failed to generate checksums: %w", err)
		}

		<-done

		if _, err := bw.WriteString(formatStatsFooter(generator.Stats(), hasError)); err != nil && hasError == nil {
			hasError = fmt.Errorf("failed to write stats footer: %w", err)
		}

		if err := bw.Flush(); err != nil && hasError == nil {
			hasError = fmt.Errorf("failed to flush buffer: %w", err)
		}

		resultCh <- GenerateStreamingResult{
			Stats:            generator.Stats(),
			IsProgressUpdate: true,
			Err:              hasError,
		}
	}()

	return resultCh, nil
}

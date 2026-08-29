package verify

import (
	"context"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

// VerifyConfig is the shared input for VerifyChecksums and its streaming variant.
type VerifyConfig struct {
	ChecksumFile   string
	Algorithm      algorithm.Algorithm
	OnFileVerified func(result result.VerifyResult)
}

// VerifyResultStats is the forward-compatible return value of VerifyChecksums.
type VerifyResultStats struct {
	Stats result.VerifierStats
}

// ValidateChecksumFile rejects paths that are missing or are not regular files, before parsing; a FIFO
// checksum file would block the parser's open read forever, so it is refused up front.
func ValidateChecksumFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat checksum file: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("checksum path %s is a directory", path)
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("checksum path %s is not a regular file", path)
	}

	return nil
}

// VerifyChecksums is the blocking entry point; it runs the pipeline to completion before returning.
func VerifyChecksums(ctx context.Context, cfg VerifyConfig) (VerifyResultStats, error) {
	if err := ValidateChecksumFile(cfg.ChecksumFile); err != nil {
		return VerifyResultStats{}, fmt.Errorf("invalid checksum file: %w", err)
	}

	if cfg.Algorithm == algorithm.Unknown {
		return VerifyResultStats{}, algorithm.ErrAlgorithmNotSpecified
	}

	verifier := NewVerifier(ctx, cfg.ChecksumFile, cfg.Algorithm)
	verifier.Start()

	var hasError error

	for res := range verifier.Results() {
		if cfg.OnFileVerified != nil {
			cfg.OnFileVerified(res)
		}

		verifier.MarkVerified(res.Status)
	}

	if err := verifier.Wait(); err != nil {
		hasError = fmt.Errorf("verification failed: %w", err)
	}

	return VerifyResultStats{Stats: verifier.Stats()}, hasError
}

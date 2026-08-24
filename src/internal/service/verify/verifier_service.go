package verify

import (
	"context"
	"fmt"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"os"
)

type VerifyConfig struct {
	ChecksumFile   string
	Algorithm      algorithm.Algorithm
	OnFileVerified func(result result.VerifyResult)
}

type VerifyResultStats struct {
	Stats result.VerifierStats
}

func ValidateChecksumFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat checksum file: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("checksum path is not a file")
	}

	return nil
}

func VerifyChecksums(ctx context.Context, cfg VerifyConfig) (VerifyResultStats, error) {
	if err := ValidateChecksumFile(cfg.ChecksumFile); err != nil {
		return VerifyResultStats{}, fmt.Errorf("invalid checksum file: %w", err)
	}

	if cfg.Algorithm == algorithm.Unknown {
		return VerifyResultStats{}, fmt.Errorf("algorithm not specified")
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

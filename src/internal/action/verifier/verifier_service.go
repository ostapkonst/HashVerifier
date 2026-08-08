package verifier

import (
	"context"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

type VerifyConfig struct {
	ChecksumFile   string
	Extension      string
	OnFileVerified func(result checksum.VerifyResult)
}

type VerifyResultStats struct {
	Stats checksum.VerifierStats
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

func ResolveAlgorithm(filename, extension string) (checksum.Algorithm, error) {
	if extension != "" {
		return checksum.AlgorithmFromExtension(extension)
	}

	return checksum.AlgorithmFromExtension(filename)
}

func VerifyChecksums(ctx context.Context, cfg VerifyConfig) (VerifyResultStats, error) {
	if err := ValidateChecksumFile(cfg.ChecksumFile); err != nil {
		return VerifyResultStats{}, fmt.Errorf("invalid checksum file: %w", err)
	}

	algo, err := ResolveAlgorithm(cfg.ChecksumFile, cfg.Extension)
	if err != nil {
		return VerifyResultStats{}, fmt.Errorf("unsupported algorithm: %w", err)
	}

	verifier := NewVerifier(ctx, cfg.ChecksumFile, algo)
	verifier.Start()

	var hasError error

	for res := range verifier.Results() {
		if cfg.OnFileVerified != nil {
			cfg.OnFileVerified(res)
		}
	}

	if err := verifier.Wait(); err != nil {
		hasError = fmt.Errorf("verification failed: %w", err)
	}

	return VerifyResultStats{Stats: verifier.Stats()}, hasError
}

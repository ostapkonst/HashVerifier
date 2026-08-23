package hasher

import (
	"context"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

type HashConfig struct {
	FilePath   string
	Algorithms []checksum.Algorithm
}

type HashResult struct {
	Hashes map[checksum.Algorithm]string
}

func ValidateFilePath(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory")
	}

	return nil
}

func HashFile(ctx context.Context, cfg HashConfig) (HashResult, error) {
	if err := ValidateFilePath(cfg.FilePath); err != nil {
		return HashResult{}, fmt.Errorf("invalid file path: %w", err)
	}

	if len(cfg.Algorithms) == 0 {
		return HashResult{}, fmt.Errorf("no algorithms specified")
	}

	speedTracker := checksum.NewSpeedTracker()
	hashCalc := checksum.NewMultiHashCalculator(cfg.FilePath, cfg.Algorithms, speedTracker)

	multiResult, err := hashCalc.Calculate(ctx)
	if err != nil {
		return HashResult{}, fmt.Errorf("failed to calculate hash: %w", err)
	}

	return HashResult{
		Hashes: multiResult.Hashes,
	}, nil
}

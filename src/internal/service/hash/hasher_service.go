package hash

import (
	"context"
	"fmt"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"os"
)

type HashConfig struct {
	FilePath   string
	Algorithms []algorithm.Algorithm
}

type HashResult struct {
	Hashes map[algorithm.Algorithm]string
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

	speedTracker := result.NewSpeedTracker()
	hashCalc := hashfn.NewMultiHashCalculator(cfg.FilePath, cfg.Algorithms, speedTracker)

	multiResult, err := hashCalc.Calculate(ctx)
	if err != nil {
		return HashResult{}, fmt.Errorf("failed to calculate hash: %w", err)
	}

	return HashResult{
		Hashes: multiResult.Hashes,
	}, nil
}

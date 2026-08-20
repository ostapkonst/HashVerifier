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
	Hash      string
	Algorithm checksum.Algorithm
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

func HashFile(ctx context.Context, cfg HashConfig) ([]HashResult, error) {
	if err := ValidateFilePath(cfg.FilePath); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	if len(cfg.Algorithms) == 0 {
		return nil, fmt.Errorf("no algorithms specified")
	}

	speedTracker := checksum.NewSpeedTracker()
	hashCalc := checksum.NewMultiHashCalculator(cfg.FilePath, cfg.Algorithms, speedTracker)

	multiResult, err := hashCalc.Calculate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	results := make([]HashResult, 0, len(cfg.Algorithms))
	for _, algoType := range cfg.Algorithms {
		results = append(results, HashResult{
			Hash:      multiResult.Hashes[algoType],
			Algorithm: algoType,
		})
	}

	return results, nil
}

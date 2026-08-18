package hasher

import (
	"context"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

type HashConfig struct {
	FilePath   string
	Algorithms []string
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

func ParseAlgorithms(algorithms []string) ([]checksum.Algorithm, error) {
	if len(algorithms) == 0 {
		return nil, fmt.Errorf("no algorithms specified")
	}

	result := make([]checksum.Algorithm, 0, len(algorithms))

	for _, algoStr := range algorithms {
		algoType, err := checksum.AlgorithmFromExtension(algoStr)
		if err != nil {
			return nil, fmt.Errorf("unsupported algorithm %s: %w", algoStr, err)
		}

		result = append(result, algoType)
	}

	return result, nil
}

func HashFile(ctx context.Context, cfg HashConfig) ([]HashResult, error) {
	if err := ValidateFilePath(cfg.FilePath); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	algorithms, err := ParseAlgorithms(cfg.Algorithms)
	if err != nil {
		return nil, fmt.Errorf("failed to parse algorithms: %w", err)
	}

	speedTracker := checksum.NewSpeedTracker()
	hashCalc := checksum.NewMultiHashCalculator(cfg.FilePath, algorithms, speedTracker)

	multiResult, err := hashCalc.Calculate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	results := make([]HashResult, 0, len(algorithms))
	for _, algoType := range algorithms {
		results = append(results, HashResult{
			Hash:      multiResult.Hashes[algoType],
			Algorithm: algoType,
		})
	}

	return results, nil
}

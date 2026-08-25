// Package hash contains the use-case orchestration for hashing a single file with one or more algorithms.
package hash

import (
	"context"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

// HashConfig holds the file path and algorithm set hashed together in one pass.
type HashConfig struct {
	FilePath   string
	Algorithms []algorithm.Algorithm
}

// HashResult groups the digests returned by HashFile, keyed by algorithm.
type HashResult struct {
	Hashes map[algorithm.Algorithm]string
}

// ValidateFilePath rejects paths that do not exist or are not regular files, before the read begins.
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

// HashFile blocks computing all cfg.Algorithms on cfg.FilePath in a single read pass.
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

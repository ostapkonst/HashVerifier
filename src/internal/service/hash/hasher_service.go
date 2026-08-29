// Package hash contains the use-case orchestration for hashing a single file with one or more algorithms.
package hash

import (
	"context"
	"errors"
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

// ErrNoAlgorithms is returned when HashConfig carries no algorithms to hash with.
var ErrNoAlgorithms = errors.New("no algorithms specified")

// ValidateFilePath rejects paths that do not exist or are not regular files, before the read begins; a FIFO
// input would block the calculator's open read forever, so it is refused up front.
func ValidateFilePath(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
		return err
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("path %s is a directory", path)
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("path %s is not a regular file", path)
	}

	return nil
}

// HashFile blocks computing all cfg.Algorithms on cfg.FilePath in a single read pass.
func HashFile(ctx context.Context, cfg HashConfig) (HashResult, error) {
	if err := ValidateFilePath(cfg.FilePath); err != nil {
		return HashResult{}, fmt.Errorf("invalid file path: %w", err)
	}

	if len(cfg.Algorithms) == 0 {
		return HashResult{}, ErrNoAlgorithms
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

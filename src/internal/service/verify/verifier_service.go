package verify

import (
	"context"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

// VerifyConfig is the input for the blocking VerifyChecksums; the streaming path has its own VerifyStreamingConfig.
type VerifyConfig struct {
	ChecksumFile   string
	Algorithm      algorithm.Algorithm
	OnFileVerified func(result result.VerifyResult)
}

// VerifyResultStats carries the aggregate counters of a completed VerifyChecksums run.
type VerifyResultStats struct {
	Stats result.VerifierStats
}

// ValidateChecksumFile rejects paths that are missing, are directories, or are not regular files,
// before parsing; a FIFO/socket/device checksum file would block the parser's open read forever,
// so it is refused up front.
func ValidateChecksumFile(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
		return err
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("checksum path is a directory: %s", path)
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("checksum path is not a regular file: %s", path)
	}

	return nil
}

// VerifyChecksums is the blocking entry point; it runs the pipeline to completion before returning.
// The returned error carries the run's terminal failure, cancellation included; mismatches and
// unreadable files are counted in Stats and do not surface as an error.
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
		hasError = err
	}

	return VerifyResultStats{Stats: verifier.Stats()}, hasError
}

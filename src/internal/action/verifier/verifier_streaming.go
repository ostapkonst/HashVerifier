package verifier

import (
	"context"
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

const statsUpdateInterval = 50 * time.Millisecond

type VerifyStreamingResult struct {
	Result           checksum.VerifyResult
	Stats            checksum.VerifierStats
	Err              error
	IsProgressUpdate bool
}

type VerifyStreamingConfig struct {
	ChecksumFile string
	Algorithm    string
}

func VerifyChecksumsStreaming(ctx context.Context, cfg VerifyStreamingConfig) (<-chan VerifyStreamingResult, error) {
	if err := ValidateChecksumFile(cfg.ChecksumFile); err != nil {
		return nil, fmt.Errorf("invalid checksum file: %w", err)
	}

	algo, err := ResolveAlgorithm(cfg.ChecksumFile, cfg.Algorithm)
	if err != nil {
		return nil, fmt.Errorf("unsupported algorithm: %w", err)
	}

	resultCh := make(chan VerifyStreamingResult, 1)

	go func() {
		defer close(resultCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		verifier := NewVerifier(ctx, cfg.ChecksumFile, algo)
		verifier.Start()

		var hasError error

		done := make(chan struct{})

		go func() {
			defer close(done)

			ticker := time.NewTicker(statsUpdateInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					select {
					case resultCh <- VerifyStreamingResult{
						Stats:            verifier.Stats(),
						IsProgressUpdate: true,
					}:
					default:
					}
				}
			}
		}()

		for res := range verifier.Results() {
			resultCh <- VerifyStreamingResult{
				Result: res,
				Stats:  verifier.Stats(),
			}
		}

		cancel()

		if err := verifier.Wait(); err != nil {
			hasError = fmt.Errorf("verification failed: %w", err)
		}

		<-done

		resultCh <- VerifyStreamingResult{
			Stats:            verifier.Stats(),
			IsProgressUpdate: true,
			Err:              hasError,
		}
	}()

	return resultCh, nil
}

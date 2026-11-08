package verify

import (
	"context"
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
)

// VerifyStreamingResult is one streaming item: a per-file result, a progress tick, or a terminal error.
// IsProgressUpdate marks the stats-only snapshots; the terminal error item carries the flag too,
// so Err is what tells the two apart.
type VerifyStreamingResult struct {
	Result           result.VerifyResult
	Stats            result.VerifierStats
	Err              error
	IsProgressUpdate bool
}

// VerifyStreamingConfig feeds the streaming variant; OnFileVerified is absent because
// consumers subscribe via the channel.
type VerifyStreamingConfig struct {
	ChecksumFile string
	Algorithm    algorithm.Algorithm
}

// VerifyChecksumsStreaming returns a channel of progress events; close of the channel signals completion.
// The returned error covers start-up failures only; the run's terminal error, cancellation
// included, arrives in Err on the last item before the channel closes.
func VerifyChecksumsStreaming(ctx context.Context, cfg VerifyStreamingConfig) (<-chan VerifyStreamingResult, error) {
	if err := ValidateChecksumFile(cfg.ChecksumFile); err != nil {
		return nil, fmt.Errorf("invalid checksum file: %w", err)
	}

	if cfg.Algorithm == algorithm.Unknown {
		return nil, algorithm.ErrAlgorithmNotSpecified
	}

	resultCh := make(chan VerifyStreamingResult, 1)

	crash.Go("verify.streaming.consumer", func() {
		defer close(resultCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		verifier := NewVerifier(ctx, cfg.ChecksumFile, cfg.Algorithm)
		verifier.Start()

		var hasError error

		done := make(chan struct{})

		crash.Go("verify.streaming.progressTicker", func() {
			defer close(done)

			sendStats := func() bool {
				select {
				case resultCh <- VerifyStreamingResult{
					Stats:            verifier.Stats(),
					IsProgressUpdate: true,
				}:
					return true
				case <-ctx.Done():
					return false
				}
			}

			// Immediate snapshot so the UI shows progress without waiting one interval.
			if !sendStats() {
				return
			}

			ticker := time.NewTicker(result.ProgressUpdateInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !sendStats() {
						return
					}
				}
			}
		})

		for res := range verifier.Results() {
			verifier.MarkVerified(res.Status)

			resultCh <- VerifyStreamingResult{
				Result: res,
				Stats:  verifier.Stats(),
			}
		}

		cancel()

		if err := verifier.Wait(); err != nil {
			hasError = err
		}

		<-done

		resultCh <- VerifyStreamingResult{
			Stats:            verifier.Stats(),
			IsProgressUpdate: true,
			Err:              hasError,
		}
	})

	return resultCh, nil
}

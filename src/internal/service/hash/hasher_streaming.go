package hash

import (
	"context"
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
)

const hashProgressInterval = 50 * time.Millisecond

// HashStreamingResult is one item produced by HashFileStreaming; items may be a progress tick or a terminal result.
type HashStreamingResult struct {
	Result           HashResult
	Progress         float64
	Err              error
	IsProgressUpdate bool
}

// HashFileStreaming returns a channel of progress events; close of the channel signals completion.
func HashFileStreaming(ctx context.Context, cfg HashConfig) (<-chan HashStreamingResult, error) {
	if err := ValidateFilePath(cfg.FilePath); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	if len(cfg.Algorithms) == 0 {
		return nil, ErrNoAlgorithms
	}

	resultCh := make(chan HashStreamingResult, 1)

	crash.Go("hash.streaming.consumer", func() {
		defer close(resultCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		speedTracker := result.NewSpeedTracker()
		hashCalc := hashfn.NewMultiHashCalculator(cfg.FilePath, cfg.Algorithms, speedTracker)

		var hasError error

		done := make(chan struct{})

		crash.Go("hash.streaming.progressTicker", func() {
			defer close(done)

			sendProgress := func() bool {
				select {
				case resultCh <- HashStreamingResult{
					Progress:         hashCalc.Progress(),
					IsProgressUpdate: true,
				}:
					return true
				case <-ctx.Done():
					return false
				}
			}

			// Immediate snapshot so the UI shows progress without waiting one interval.
			if !sendProgress() {
				return
			}

			ticker := time.NewTicker(hashProgressInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !sendProgress() {
						return
					}
				}
			}
		})

		multiResult, err := hashCalc.Calculate(ctx)

		cancel()
		<-done

		if err != nil {
			hasError = err
		}

		resultCh <- HashStreamingResult{
			Result: HashResult{
				Hashes: multiResult.Hashes,
			},
			Progress: hashCalc.Progress(),
			Err:      hasError,
		}
	})

	return resultCh, nil
}

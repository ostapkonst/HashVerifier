package hash

import (
	"context"
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
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

	go func() {
		defer close(resultCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		speedTracker := result.NewSpeedTracker()
		hashCalc := hashfn.NewMultiHashCalculator(cfg.FilePath, cfg.Algorithms, speedTracker)

		var hasError error

		done := make(chan struct{})

		go func() {
			defer close(done)

			ticker := time.NewTicker(hashProgressInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					progress := hashCalc.Progress()
					select {
					case resultCh <- HashStreamingResult{
						Progress:         progress,
						IsProgressUpdate: true,
					}:
					default:
					}
				}
			}
		}()

		multiResult, err := hashCalc.Calculate(ctx)

		cancel()
		<-done

		if err != nil {
			hasError = fmt.Errorf("failed to calculate hash: %w", err)
		}

		resultCh <- HashStreamingResult{
			Result: HashResult{
				Hashes: multiResult.Hashes,
			},
			Progress: hashCalc.Progress(),
			Err:      hasError,
		}
	}()

	return resultCh, nil
}

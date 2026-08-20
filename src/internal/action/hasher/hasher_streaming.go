package hasher

import (
	"context"
	"fmt"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

const hashProgressInterval = 50 * time.Millisecond

type HashStreamingResult struct {
	Result           HashResult
	Progress         float64
	Algorithm        checksum.Algorithm
	Err              error
	IsProgressUpdate bool
}

func HashFileStreaming(ctx context.Context, cfg HashConfig) (<-chan HashStreamingResult, error) {
	if err := ValidateFilePath(cfg.FilePath); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	if len(cfg.Algorithms) == 0 {
		return nil, fmt.Errorf("no algorithms specified")
	}

	resultCh := make(chan HashStreamingResult, 1)

	go func() {
		defer close(resultCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		speedTracker := checksum.NewSpeedTracker()
		hashCalc := checksum.NewMultiHashCalculator(cfg.FilePath, cfg.Algorithms, speedTracker)

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

		for _, algoType := range cfg.Algorithms {
			resultCh <- HashStreamingResult{ // IsProgressUpdate = false т. к. это финальный результат
				Result: HashResult{
					Hash:      multiResult.Hashes[algoType],
					Algorithm: algoType,
				},
				Progress:  hashCalc.Progress(),
				Algorithm: algoType,
				Err:       hasError,
			}
		}
	}()

	return resultCh, nil
}

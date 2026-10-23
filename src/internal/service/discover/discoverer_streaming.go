package discover

import (
	"context"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
)

const statsUpdateInterval = 50 * time.Millisecond

// DiscoverStreamingResult is one streaming item: a per-file result, a progress tick, or a terminal error.
type DiscoverStreamingResult struct {
	Result           result.DiscoverResult
	Stats            result.DiscovererStats
	Err              error
	IsProgressUpdate bool
}

// DiscoverStreamingConfig feeds the streaming variant; consumers subscribe via the channel.
type DiscoverStreamingConfig struct {
	Sources             []string
	Filter              embedded.FilterMode
	FollowSymbolicLinks bool
}

// DiscoverStreaming returns a channel of progress events; close of the channel signals completion.
func DiscoverStreaming(ctx context.Context, cfg DiscoverStreamingConfig) (<-chan DiscoverStreamingResult, error) {
	if err := ValidateSources(cfg.Sources); err != nil {
		return nil, err
	}

	resultCh := make(chan DiscoverStreamingResult, 1)

	crash.Go("discover.streaming.consumer", func() {
		defer close(resultCh)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		discoverer := NewDiscoverer(ctx, cfg.Sources, cfg.Filter, cfg.FollowSymbolicLinks)
		discoverer.Start()

		var hasError error

		done := make(chan struct{})

		crash.Go("discover.streaming.progressTicker", func() {
			defer close(done)

			sendStats := func() bool {
				select {
				case resultCh <- DiscoverStreamingResult{
					Stats:            discoverer.Stats(),
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

			ticker := time.NewTicker(statsUpdateInterval)
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

		for res := range discoverer.Results() {
			discoverer.MarkStatus(res.Status)

			resultCh <- DiscoverStreamingResult{
				Result: res,
				Stats:  discoverer.Stats(),
			}
		}

		cancel()

		if err := discoverer.Wait(); err != nil {
			hasError = err
		}

		<-done

		resultCh <- DiscoverStreamingResult{
			Stats:            discoverer.Stats(),
			IsProgressUpdate: true,
			Err:              hasError,
		}
	})

	return resultCh, nil
}

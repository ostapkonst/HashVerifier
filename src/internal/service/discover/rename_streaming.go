package discover

import (
	"context"
	"time"

	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
)

// RenameStreamingResult is one streaming item: a per-file outcome or a progress/terminal update.
type RenameStreamingResult struct {
	Outcome          RenameOutcome
	Stats            result.RenameStats
	Err              error
	IsProgressUpdate bool
}

// RenameStreaming renames actions sequentially and streams progress; close of the channel signals
// completion. Cancellation is checked before each rename, so renames already applied are kept.
func RenameStreaming(ctx context.Context, actions []embedded.RenameAction) <-chan RenameStreamingResult {
	resultCh := make(chan RenameStreamingResult, 1)

	crash.Go("discover.rename.streaming", func() {
		defer close(resultCh)

		start := time.Now()
		stats := result.NewRenameStats(len(actions))
		stats.CurrentFileOrStatus = "renaming files..."
		stats.Speed = renameSpeed(stats, start)

		// Immediate snapshot so the UI shows progress before the first file.
		resultCh <- RenameStreamingResult{Stats: stats, IsProgressUpdate: true}

		var hasError error

		for _, action := range actions {
			select {
			case <-ctx.Done():
				hasError = ctx.Err()
			default:
			}

			if hasError != nil {
				break
			}

			stats.CurrentFileOrStatus = action.From

			err := applyRename(action)
			if err != nil {
				stats.Failed++
			} else {
				stats.Renamed++
			}

			stats.Speed = renameSpeed(stats, start)

			resultCh <- RenameStreamingResult{
				Outcome: RenameOutcome{Action: action, Err: err},
				Stats:   stats,
			}
		}

		stats.CurrentFileOrStatus = "ready to go..."
		stats.Speed = renameSpeed(stats, start)

		resultCh <- RenameStreamingResult{
			Stats:            stats,
			IsProgressUpdate: true,
			Err:              hasError,
		}
	})

	return resultCh
}

// renameSpeed returns processed renames per second since start; zero for an empty run.
func renameSpeed(stats result.RenameStats, start time.Time) float64 {
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0
	}

	return float64(stats.Renamed+stats.Failed) / elapsed
}

// RenameFiles is the blocking entry point; it applies actions to completion, reporting each
// outcome to onOutcome, and returns the final statistics plus the terminal error (cancellation).
func RenameFiles(ctx context.Context, actions []embedded.RenameAction, onOutcome func(RenameOutcome)) (result.RenameStats, error) {
	var (
		stats    result.RenameStats
		hasError error
	)

	for item := range RenameStreaming(ctx, actions) {
		stats = item.Stats

		if item.IsProgressUpdate {
			if item.Err != nil {
				hasError = item.Err
			}

			continue
		}

		if onOutcome != nil {
			onOutcome(item.Outcome)
		}
	}

	return stats, hasError
}

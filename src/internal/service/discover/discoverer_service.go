package discover

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

// DiscoverConfig is the input for the blocking DiscoverFiles.
type DiscoverConfig struct {
	Sources             []string
	FollowSymbolicLinks bool
	SkipHidden          bool
	OnFileDiscovered    func(result result.DiscoverResult)
}

// DiscoverResultStats carries the aggregate counters of a completed DiscoverFiles run.
type DiscoverResultStats struct {
	Stats result.DiscovererStats
}

// ValidateSources rejects an empty source list and any source that cannot be stat-ed; it runs
// before a run starts so an obviously invalid selection never launches a goroutine.
func ValidateSources(sources []string) error {
	if len(sources) == 0 {
		return errors.New("no sources selected")
	}

	for _, src := range sources {
		if _, err := os.Stat(src); err != nil {
			// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
			return err
		}
	}

	return nil
}

// DiscoverFiles is the blocking entry point; it runs the pipeline to completion before returning.
// The returned error carries the run's terminal failure, cancellation included; mismatches and failed
// files are counted in Stats and do not surface as an error.
func DiscoverFiles(ctx context.Context, cfg DiscoverConfig) (DiscoverResultStats, error) {
	if err := ValidateSources(cfg.Sources); err != nil {
		return DiscoverResultStats{}, fmt.Errorf("invalid sources: %w", err)
	}

	discoverer := NewDiscoverer(ctx, cfg.Sources, cfg.FollowSymbolicLinks, cfg.SkipHidden)
	discoverer.Start()

	var hasError error

	for res := range discoverer.Results() {
		if cfg.OnFileDiscovered != nil {
			cfg.OnFileDiscovered(res)
		}

		discoverer.MarkDiscovered(res.Status)
	}

	if err := discoverer.Wait(); err != nil {
		hasError = err
	}

	return DiscoverResultStats{Stats: discoverer.Stats()}, hasError
}

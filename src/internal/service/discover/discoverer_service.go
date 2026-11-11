package discover

import (
	"context"
	"fmt"
	"os"

	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/scan"
)

// DiscoverConfig is the input for the blocking DiscoverFiles.
type DiscoverConfig struct {
	Sources             []string
	FollowSymbolicLinks bool
	SkipHidden          bool
	FilesToScan         scan.FilesToScan
	Action              scan.Action
	OnFileDiscovered    func(result result.DiscoverResult)
}

// DiscoverResultStats carries the aggregate counters of a completed DiscoverFiles run.
type DiscoverResultStats struct {
	Stats result.DiscovererStats
}

// ValidateSources rejects any source that cannot be stat-ed; it runs before a run starts so an
// obviously invalid selection never launches a goroutine. An empty list is valid: it means there
// is nothing to scan, and the run completes with zero results.
func ValidateSources(sources []string) error {
	for _, src := range sources {
		if _, err := os.Stat(src); err != nil {
			// fs.PathError already embeds the op ("stat") and path; another prefix would double them.
			return err
		}
	}

	return nil
}

// resolvePolicies fills empty policy fields with defaults and rejects unknown values so a
// programmatic caller cannot start a run with an undefined scope or action.
func resolvePolicies(filesToScan scan.FilesToScan, action scan.Action) (scan.FilesToScan, scan.Action, error) {
	if filesToScan == "" {
		filesToScan = scan.DefaultFilesToScan
	}

	if action == "" {
		action = scan.DefaultAction
	}

	if !filesToScan.Valid() {
		return "", "", fmt.Errorf("invalid files to scan: %s", filesToScan)
	}

	if !action.Valid() {
		return "", "", fmt.Errorf("invalid action: %s", action)
	}

	return filesToScan, action, nil
}

// DiscoverFiles is the blocking entry point; it runs the pipeline to completion before returning.
// The returned error carries the run's terminal failure, cancellation included; mismatches and failed
// files are counted in Stats and do not surface as an error.
func DiscoverFiles(ctx context.Context, cfg DiscoverConfig) (DiscoverResultStats, error) {
	if err := ValidateSources(cfg.Sources); err != nil {
		return DiscoverResultStats{}, fmt.Errorf("invalid sources: %w", err)
	}

	filesToScan, action, err := resolvePolicies(cfg.FilesToScan, cfg.Action)
	if err != nil {
		return DiscoverResultStats{}, err
	}

	discoverer := NewDiscoverer(ctx, cfg.Sources, cfg.FollowSymbolicLinks, cfg.SkipHidden, filesToScan, action)
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

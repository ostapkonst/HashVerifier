// Package discover contains the use-case orchestration for scanning sources for files whose names
// embed a CRC-32 and comparing it with the checksum computed from the file contents.
package discover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/embedded"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/hidden"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
)

// DiscovererStatusType marks whether a Discoverer has begun or completed its run.
type DiscovererStatusType int

// DiscovererStatusType values: Finished is both the initial and terminal state, Started means a run is in flight.
const (
	DiscovererStatusFinished DiscovererStatusType = iota
	DiscovererStatusStarted
)

// Discoverer walks the configured sources, hashes each file with CRC-32, and compares the digest
// with the checksum embedded in the file name when present.
//
// A Discoverer instance is intended for a single run: the internal ctx is canceled when the run
// completes, so callers needing multiple runs should construct a fresh Discoverer per call.
type Discoverer struct {
	rwm    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	sources             []string
	filter              embedded.FilterMode
	followSymbolicLinks bool
	skipHidden          bool

	stats                   result.DiscovererStats
	currFileHashingProgress atomic.Value
	speedTracker            *result.SpeedTracker

	status DiscovererStatusType

	err      chan error
	resultCh chan result.DiscoverResult
	done     chan struct{}
}

// NewDiscoverer constructs a Discoverer; Start kicks off the walk-and-hash run, Wait/Results consume it.
func NewDiscoverer(
	ctx context.Context,
	sources []string,
	filter embedded.FilterMode,
	followSymbolicLinks, skipHidden bool,
) *Discoverer {
	ctx, cancel := context.WithCancel(ctx)

	d := &Discoverer{
		ctx:                 ctx,
		cancel:              cancel,
		sources:             sources,
		filter:              filter,
		followSymbolicLinks: followSymbolicLinks,
		skipHidden:          skipHidden,
		status:              DiscovererStatusFinished,
		speedTracker:        result.NewSpeedTracker(),
	}

	return d
}

// Start launches the walk-and-hash goroutine; a no-op while a run is in flight.
func (d *Discoverer) Start() {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	if d.status != DiscovererStatusFinished {
		return
	}

	d.resultCh = make(chan result.DiscoverResult, 1)
	d.done = make(chan struct{})
	d.err = make(chan error, 1)

	d.status = DiscovererStatusStarted

	d.stats = result.NewDiscovererStats()
	d.currFileHashingProgress.Store(func() float64 { return 0 })
	d.speedTracker.Reset()

	crash.Go("discover.run", d.run)
}

// Wait blocks until the current run completes and returns the terminal error (or nil on success).
func (d *Discoverer) Wait() error {
	<-d.done
	return <-d.err
}

// Stats returns the current aggregate progress snapshot including file-hash progress and rolling speed.
func (d *Discoverer) Stats() result.DiscovererStats {
	fileHashProgress := d.currFileHashingProgress.Load().(func() float64)

	d.rwm.RLock()
	defer d.rwm.RUnlock()

	stats := d.stats
	stats.FileHashingProgress = fileHashProgress()
	stats.Speed = d.speedTracker.Speed()

	return stats
}

// Results returns the receive-only channel of per-file DiscoverResult events for the current run;
// it is closed when the run finishes.
func (d *Discoverer) Results() <-chan result.DiscoverResult {
	return d.resultCh
}

// MarkStatus classifies status into the Matched / Mismatch / Failed / NoChecksum / Skipped counters;
// called by the consumer after each per-file comparison.
func (d *Discoverer) MarkStatus(status result.DiscoverStatusType) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	switch status {
	case result.DiscoverMatched:
		d.stats.Matched++
	case result.DiscoverMismatch:
		d.stats.Mismatch++
	case result.DiscoverFailed:
		d.stats.Failed++
	case result.DiscoverNoChecksum:
		d.stats.NoChecksum++
	case result.DiscoverSkipped:
		d.stats.Skipped++
	}
}

func (d *Discoverer) updateCurrentFileOrStatus(file string) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	d.stats.CurrentFileOrStatus = file
}

func (d *Discoverer) updateStatsTotal(totalFiles int) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	d.stats.TotalFiles = totalFiles
}

func (d *Discoverer) run() {
	defer func() {
		d.updateCurrentFileOrStatus("ready to go...")
		d.speedTracker.Reset()
	}()
	defer func() {
		d.rwm.Lock()
		defer d.rwm.Unlock()

		d.status = DiscovererStatusFinished
	}()

	defer close(d.done)
	defer close(d.err)
	defer close(d.resultCh)
	defer d.cancel()

	d.updateCurrentFileOrStatus("forming a list of files for discovery...")

	files, hiddenFiles, err := d.collectEntries()
	if err != nil {
		d.err <- fmt.Errorf("discovering sources: %w", err)
		return
	}

	total := 0

	for _, path := range hiddenFiles {
		if d.filter.Accepts(filepath.Base(path)) {
			total++
		}
	}

	for _, path := range files {
		if d.filter.Accepts(filepath.Base(path)) {
			total++
		}
	}

	d.updateStatsTotal(total)

	for _, path := range hiddenFiles {
		if !d.filter.Accepts(filepath.Base(path)) {
			continue
		}

		if !d.emit(result.DiscoverResult{
			Path:   path,
			Status: result.DiscoverSkipped,
			Err:    hidden.ErrHidden,
		}) {
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		}
	}

	for _, path := range files {
		select {
		case <-d.ctx.Done():
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		default:
		}

		base := filepath.Base(path)
		if !d.filter.Accepts(base) {
			continue
		}

		res, ok := d.discoverOne(path, base)
		if !ok {
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		}

		if !d.emit(res) {
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		}
	}
}

// discoverOne hashes one file and classifies it against the checksum embedded in base; a hash
// failure (other than cancellation) is reported as Failed.
func (d *Discoverer) discoverOne(path, base string) (result.DiscoverResult, bool) {
	d.updateCurrentFileOrStatus(path)

	hashCalc := hashfn.NewHashCalculator(path, algorithm.CRC32, d.speedTracker)
	d.currFileHashingProgress.Store(hashCalc.Progress)

	hashResult, err := hashCalc.Calculate(d.ctx)
	if err != nil {
		if errs.IsContextDone(err) {
			return result.DiscoverResult{}, false
		}

		return result.DiscoverResult{
			Path:   path,
			Status: result.DiscoverFailed,
			Err:    err,
		}, true
	}

	res := result.DiscoverResult{
		Path:       path,
		ActualHash: strings.ToLower(hashResult.Hash),
		ReadBytes:  hashResult.ReadBytes,
	}

	_, checksum, _, ok := embedded.Split(base)
	if !ok {
		res.Status = result.DiscoverNoChecksum

		return res, true
	}

	res.EmbeddedHash = strings.ToLower(checksum)

	if strings.EqualFold(hashResult.Hash, checksum) {
		res.Status = result.DiscoverMatched
	} else {
		res.Status = result.DiscoverMismatch
	}

	return res, true
}

// emit sends res to the result channel, returning false when the context is canceled first.
func (d *Discoverer) emit(res result.DiscoverResult) bool {
	select {
	case d.resultCh <- res:
		return true
	case <-d.ctx.Done():
		return false
	}
}

// collectEntries resolves the configured sources into deduplicated absolute file paths, with
// hidden files classified separately so they can be reported as skipped; a directory source is
// walked, a file source is taken as-is. Deduplication is by absolute path so overlapping sources
// are hashed once. Entries the walk cannot enumerate are logged and ignored, matching generate.
func (d *Discoverer) collectEntries() (files, hiddenFiles []string, err error) {
	seen := make(map[string]bool)

	abs := func(p string) string {
		resolved, aerr := filepath.Abs(p)
		if aerr != nil {
			return p
		}

		return resolved
	}

	for _, src := range d.sources {
		info, serr := os.Stat(src)
		if serr != nil {
			return nil, nil, serr
		}

		if info.IsDir() {
			wr, werr := walk.WalkDir(d.ctx, src, d.followSymbolicLinks, false)
			if werr != nil {
				return nil, nil, werr
			}

			var checker *fs.HiddenChecker
			if d.skipHidden {
				checker = fs.NewHiddenChecker(src)
			}

			for _, f := range wr.Files {
				p := abs(f)

				if seen[p] {
					continue
				}

				seen[p] = true

				if checker.IsHidden(relTo(src, f)) {
					hiddenFiles = append(hiddenFiles, p)

					continue
				}

				files = append(files, p)
			}

			for _, s := range wr.Skipped {
				log.Warn().Err(s.Err).Str("path", s.Path).Msg("walk: skipped entry")
			}

			continue
		}

		if info.Mode().IsRegular() {
			if p := abs(src); !seen[p] {
				seen[p] = true
				files = append(files, p)
			}
		}
	}

	return files, hiddenFiles, nil
}

// relTo returns path relative to root, falling back to path itself when Rel fails.
func relTo(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}

	return rel
}

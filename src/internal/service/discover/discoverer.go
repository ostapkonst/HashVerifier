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

// Discoverer walks the configured sources, hashes each file whose name embeds a CRC-32, and
// compares the digest with that checksum.
// Stats and MarkDiscovered are mutex-protected; Wait/Results read channels installed by Start
// and are safe to call from any goroutine once Start has returned.
//
// A Discoverer instance is intended for a single run: the internal ctx is canceled when the run
// completes, so callers needing multiple runs should construct a fresh Discoverer per call.
type Discoverer struct {
	rwm    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	sources             []string
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
	followSymbolicLinks, skipHidden bool,
) *Discoverer {
	ctx, cancel := context.WithCancel(ctx)

	d := &Discoverer{
		ctx:                 ctx,
		cancel:              cancel,
		sources:             sources,
		followSymbolicLinks: followSymbolicLinks,
		skipHidden:          skipHidden,
		status:              DiscovererStatusFinished,
		speedTracker:        result.NewSpeedTracker(),
	}

	return d
}

// discoveredFile is one checksum-bearing file found under the configured sources.
type discoveredFile struct {
	Path     string
	Checksum string
}

// Start launches the walk-and-hash goroutine; a no-op while a run is in flight.
// The internal ctx is canceled when the run ends, successful or failed, so a second Start
// on the same instance re-launches a goroutine that fails immediately with ctx.Err(). Callers
// needing multiple runs should construct a fresh Discoverer per run.
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

// MarkDiscovered classifies status into the Matched / Mismatch / Failed / Skipped counters;
// called by the consumer after each per-file result. Panics on a status it does not recognize.
func (d *Discoverer) MarkDiscovered(status result.DiscoverStatusType) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	switch status {
	case result.DiscoverMatched:
		d.stats.Matched++
	case result.DiscoverMismatch:
		d.stats.Mismatch++
	case result.DiscoverFailed:
		d.stats.Failed++
	case result.DiscoverSkipped:
		d.stats.Skipped++
	default:
		panic("unknown discovery status")
	}
}

func (d *Discoverer) updateCurrentFileOrStatus(file string) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	d.stats.CurrentFileOrStatus = file
}

func (d *Discoverer) updateStatsPending(totalFiles int) {
	d.rwm.Lock()
	defer d.rwm.Unlock()

	d.stats.TotalFiles = totalFiles
}

func (d *Discoverer) run() {
	defer func() {
		d.rwm.Lock()
		defer d.rwm.Unlock()

		d.status = DiscovererStatusFinished
	}()

	defer close(d.done)
	defer func() {
		// Zero the speed before consumers can read the terminal snapshot: speed is a live-run metric.
		d.updateCurrentFileOrStatus(result.StatusReady)
		d.speedTracker.Reset()
	}()
	defer close(d.err)
	defer close(d.resultCh)
	defer d.cancel()
	// Every early return must leave a terminal error in d.err (cap-1 channel, single-producer);
	// cancel exits send ctx.Err() themselves because they bypass the err-send branch.

	d.updateCurrentFileOrStatus("forming a list of files for discovery...")

	files, hiddenFiles, err := d.collectEntries()
	if err != nil {
		d.err <- fmt.Errorf("discovering sources: %w", err)
		return
	}

	d.updateStatsPending(len(hiddenFiles) + len(files))

	for _, entry := range hiddenFiles {
		if !d.emit(result.DiscoverResult{
			Path:         entry.Path,
			EmbeddedHash: entry.Checksum,
			Status:       result.DiscoverSkipped,
			ActualHash:   strings.Repeat("0", algorithm.GetHashLength(algorithm.CRC32)),
			Err:          hidden.ErrHidden,
		}) {
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		}
	}

	for _, entry := range files {
		select {
		case <-d.ctx.Done():
			d.err <- fmt.Errorf("discovering sources: %w", d.ctx.Err())
			return
		default:
		}

		res, ok := d.discoverOne(entry.Path, entry.Checksum)
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

// discoverOne hashes one checksum-bearing file and classifies it against the checksum embedded
// in its name; a hash failure (other than cancellation) is reported as Failed.
func (d *Discoverer) discoverOne(path, checksum string) (result.DiscoverResult, bool) {
	d.updateCurrentFileOrStatus(path)
	d.currFileHashingProgress.Store(func() float64 { return 0 })

	res := result.DiscoverResult{
		Path:         path,
		EmbeddedHash: checksum,
	}

	hashCalc := hashfn.NewHashCalculator(path, algorithm.CRC32, d.speedTracker)
	d.currFileHashingProgress.Store(hashCalc.Progress)

	hashResult, err := hashCalc.Calculate(d.ctx)
	if err != nil {
		if errs.IsContextDone(err) {
			return result.DiscoverResult{}, false
		}

		res.Status = result.DiscoverFailed
		res.ActualHash = strings.ToLower(hashResult.Hash)
		res.ReadBytes = hashResult.ReadBytes
		res.Err = err

		return res, true
	}

	res.ActualHash = strings.ToLower(hashResult.Hash)
	res.ReadBytes = hashResult.ReadBytes

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

// collectEntries resolves the configured sources into deduplicated checksum-bearing files,
// classifying hidden ones separately when skip-hidden is on; a directory source is walked per
// the follow-symbolic-links option, a non-directory source is taken as-is. Files without an
// embedded CRC-32 are ignored. Deduplication is by canonical path so overlapping sources are
// hashed once. Entries the walk cannot enumerate are logged and ignored, matching generate.
//
// A file found by several sources is skipped as hidden only when every one of them considers
// it hidden: a source listed explicitly is not governed by skip-hidden and wins over a broader
// source that hides the same file, so the outcome never depends on the order of the sources.
func (d *Discoverer) collectEntries() (files, hiddenFiles []discoveredFile, err error) {
	type verdict struct {
		entry  discoveredFile
		hidden bool
	}

	var entries []verdict

	byKey := make(map[string]int)

	record := func(entry discoveredFile, hidden bool) {
		key := fs.PathKey(entry.Path)

		if idx, ok := byKey[key]; ok {
			entries[idx].hidden = entries[idx].hidden && hidden

			return
		}

		byKey[key] = len(entries)
		entries = append(entries, verdict{entry: entry, hidden: hidden})
	}

	for _, source := range d.sources {
		src, aerr := filepath.Abs(source)
		if aerr != nil {
			return nil, nil, aerr
		}

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
				entry, ok := checksumEntry(f)
				if !ok {
					continue
				}

				hidden := false

				if checker != nil {
					rel, rerr := filepath.Rel(src, f)
					if rerr != nil {
						return nil, nil, fmt.Errorf("failed to calculate relative path: %w", rerr)
					}

					hidden = checker.IsHidden(rel)
				}

				record(entry, hidden)
			}

			for _, s := range wr.Skipped {
				log.Warn().Err(s.Err).Str("path", s.Path).Msg("walk: skipped entry")
			}

			continue
		}

		if entry, ok := checksumEntry(src); ok {
			record(entry, false)
		}
	}

	for _, v := range entries {
		if v.hidden {
			hiddenFiles = append(hiddenFiles, v.entry)

			continue
		}

		files = append(files, v.entry)
	}

	return files, hiddenFiles, nil
}

// checksumEntry returns the file's absolute path and embedded CRC-32, or ok=false when the
// name carries no checksum.
func checksumEntry(path string) (discoveredFile, bool) {
	checksum, ok := embedded.Checksum(filepath.Base(path))
	if !ok {
		return discoveredFile{}, false
	}

	return discoveredFile{Path: path, Checksum: strings.ToLower(checksum)}, true
}

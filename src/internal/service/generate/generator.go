// Package generate contains the use-case orchestration for generating checksum files from a directory tree.
package generate

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/hidden"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
	"github.com/ostapkonst/HashVerifier/internal/platform/crash"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
	"github.com/ostapkonst/HashVerifier/internal/platform/fs"
)

// GeneratorStatusType marks whether a Generator has begun or completed its run.
type GeneratorStatusType int

// GeneratorStatusType values: Finished is both the initial and terminal state, Started means a run is in flight.
const (
	GeneratorStatusFinished GeneratorStatusType = iota
	GeneratorStatusStarted
)

// Generator walks root and hashes every regular entry (emitting SKIPPED for excluded
// entries and entries with paths the checksum format cannot represent), streaming
// per-file results to a channel; checksum-file writing is the consumer's job.
// Stats and MarkGenerated are mutex-protected; Wait/Results read channels installed by
// Start and are safe to call from any goroutine once Start has returned.
//
// A Generator instance is intended for a single run: the internal ctx is canceled when the run completes,
// and a second Start would observe an already-canceled context. Callers needing multiple runs should
// construct a fresh Generator per call (this matches what every service entry point does today).
type Generator struct {
	rwm    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	root                    string
	outputFile              string
	algo                    algorithm.Algorithm
	dirPrefix               string
	followSymbolicLinks     bool
	sortPaths               bool
	skipHidden              bool
	excludeMatcher          *exclude.Matcher
	stats                   result.GeneratorStats
	currFileHashingProgress atomic.Value
	speedTracker            *result.SpeedTracker

	status GeneratorStatusType

	err      chan error
	resultCh chan result.GenerateResult
	done     chan struct{}
}

// NewGeneratorWithExclusions constructs a Generator; Start kicks off the walk, Wait/Results consume it.
func NewGeneratorWithExclusions(
	ctx context.Context,
	root string,
	outputFile string,
	algo algorithm.Algorithm,
	dirPrefix string,
	followSymbolicLinks bool,
	sortPaths bool,
	skipHidden bool,
	excludeMatcher *exclude.Matcher,
) *Generator {
	ctx, cancel := context.WithCancel(ctx)

	g := &Generator{
		ctx:                 ctx,
		cancel:              cancel,
		root:                root,
		outputFile:          outputFile,
		algo:                algo,
		dirPrefix:           dirPrefix,
		followSymbolicLinks: followSymbolicLinks,
		sortPaths:           sortPaths,
		skipHidden:          skipHidden,
		excludeMatcher:      excludeMatcher,
		status:              GeneratorStatusFinished,
		speedTracker:        result.NewSpeedTracker(),
	}

	return g
}

// Start kicks off the walk-and-hash goroutine that streams per-file results; a no-op while a run is in flight.
// After a successful run the internal ctx is canceled, so a second Start on the same
// instance re-launches a goroutine that fails immediately with ctx.Err(). Callers
// needing multiple runs should construct a fresh Generator per run.
func (g *Generator) Start() {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	if g.status != GeneratorStatusFinished {
		return
	}

	g.resultCh = make(chan result.GenerateResult, 1)
	g.done = make(chan struct{})
	g.err = make(chan error, 1)

	g.status = GeneratorStatusStarted

	g.stats = result.NewGeneratorStats()
	g.currFileHashingProgress.Store(func() float64 { return 0 })
	g.speedTracker.Reset()

	crash.Go("generate.run", g.run)
}

// Wait blocks until the current run completes and returns the terminal error (or nil on success).
func (g *Generator) Wait() error {
	<-g.done
	return <-g.err
}

// MarkGenerated classifies status into the Processed / Skipped / WithErrors counters;
// called by the consumer after each per-file result.
func (g *Generator) MarkGenerated(status result.GenerateStatusType) {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	switch status {
	case result.GenSuccess:
		g.stats.Processed++
	case result.GenSkipped:
		g.stats.Skipped++
	case result.GenFailed:
		g.stats.WithErrors++
	default:
		panic("unknown generation status")
	}
}

// Stats returns the current aggregate progress snapshot including file-hash progress and rolling speed.
func (g *Generator) Stats() result.GeneratorStats {
	fileHashProgress := g.currFileHashingProgress.Load().(func() float64)

	g.rwm.RLock()
	defer g.rwm.RUnlock()

	stats := g.stats
	stats.FileHashingProgress = fileHashProgress()
	stats.Speed = g.speedTracker.Speed()

	return stats
}

// Results returns the receive-only channel of per-file GenerateResult events for the
// current run; it is closed when the run finishes.
func (g *Generator) Results() <-chan result.GenerateResult {
	return g.resultCh
}

func (g *Generator) updateCurrentFileOrStatus(file string) {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	g.stats.CurrentFileOrStatus = file
}

func (g *Generator) updateStatsPending(totalFiles int) {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	g.stats.TotalFiles = totalFiles
}

func (g *Generator) run() {
	defer func() {
		g.rwm.Lock()
		defer g.rwm.Unlock()

		g.status = GeneratorStatusFinished
	}()

	defer close(g.done)
	defer func() {
		// Zero the speed before consumers can read the terminal snapshot: speed is a live-run metric.
		g.updateCurrentFileOrStatus(result.StatusReady)
		g.speedTracker.Reset()
	}()
	defer close(g.err)
	defer close(g.resultCh)
	defer g.cancel()
	// Every early return must leave a terminal error in g.err (cap-1 channel, single-producer);
	// cancel exits send ctx.Err() themselves because they bypass the err-send branch.

	g.updateCurrentFileOrStatus("forming a list of files for hashing...")

	walkResult, err := walk.WalkDir(g.ctx, g.root, g.followSymbolicLinks, g.sortPaths)
	if err != nil {
		// WalkDir already names the path; the adapter adds the generate action phrase.
		g.err <- err

		return
	}

	files := walkResult.Files
	for _, entry := range walkResult.Skipped {
		log.Warn().Err(entry.Err).Str("path", entry.Path).Msg("walk: skipped entry")
	}

	files, err = filterOutputFile(files, g.outputFile)
	if err != nil {
		g.err <- fmt.Errorf("filtering output file: %w", err)
		return
	}

	g.updateStatsPending(len(files))

	var hiddenChecker *fs.HiddenChecker
	if g.skipHidden {
		hiddenChecker = fs.NewHiddenChecker(g.root)
	}

	for _, file := range files {
		select {
		case <-g.ctx.Done():
			g.err <- fmt.Errorf("generating %s: %w", g.root, g.ctx.Err())
			return
		default:
		}

		g.updateCurrentFileOrStatus(file)
		g.currFileHashingProgress.Store(func() float64 { return 0 })

		relPath, err := filepath.Rel(g.root, file)
		if err != nil {
			g.err <- fmt.Errorf("failed to calculate relative path: %w", err)
			return
		}

		finalPath := filepath.Join(g.dirPrefix, relPath)

		if hiddenChecker.IsHidden(relPath) {
			if !g.emitSkipped(file, finalPath, hidden.ErrHidden) {
				g.err <- fmt.Errorf("generating %s: %w", g.root, g.ctx.Err())
				return
			}

			continue
		}

		if g.excludeMatcher.IsExcluded(relPath) {
			if !g.emitSkipped(file, finalPath, exclude.ErrExcludedByUser) {
				g.err <- fmt.Errorf("generating %s: %w", g.root, g.ctx.Err())
				return
			}

			continue
		}

		if perr := hashfn.ValidatePathForFormat(finalPath, g.algo); perr != nil {
			if !g.emitSkipped(file, finalPath, perr) {
				g.err <- fmt.Errorf("generating %s: %w", g.root, g.ctx.Err())
				return
			}

			continue
		}

		hashCalc := hashfn.NewHashCalculator(file, g.algo, g.speedTracker)
		g.currFileHashingProgress.Store(hashCalc.Progress)

		var fileErr error

		hashResult, err := hashCalc.Calculate(g.ctx)
		if err != nil {
			if errs.IsContextDone(err) {
				g.err <- fmt.Errorf("hashing %s: %w", file, err)
				return
			}

			fileErr = err
		}

		status := result.GenSuccess

		if fileErr != nil {
			status = result.GenFailed
		}

		select {
		case g.resultCh <- result.GenerateResult{
			FullPath:  file,
			RelPath:   finalPath,
			Hash:      strings.ToLower(hashResult.Hash),
			ReadBytes: hashResult.ReadBytes,
			Err:       fileErr,
			Status:    status,
		}:
		case <-g.ctx.Done():
			g.err <- fmt.Errorf("generating %s: %w", g.root, g.ctx.Err())
			return
		}
	}
}

// emitSkipped sends a GenSkipped result for a file that is not hashed (hidden, excluded, or
// unrepresentable). Returns false when ctx was canceled while sending, so the caller can stop.
func (g *Generator) emitSkipped(file, finalPath string, cause error) bool {
	select {
	case g.resultCh <- result.GenerateResult{
		FullPath:  file,
		RelPath:   finalPath,
		Hash:      strings.Repeat("0", algorithm.GetHashLength(g.algo)),
		ReadBytes: 0,
		Err:       cause,
		Status:    result.GenSkipped,
	}:
		return true
	case <-g.ctx.Done():
		return false
	}
}

func filterOutputFile(files []string, outputFile string) ([]string, error) {
	if outputFile == "" {
		return files, nil
	}

	filtered := make([]string, 0, len(files))

	for _, f := range files {
		same, err := fs.SameFile(f, outputFile)
		if err != nil {
			return nil, fmt.Errorf("resolve file path %s: %w", f, err)
		}

		if !same {
			filtered = append(filtered, f)
		}
	}

	return filtered, nil
}

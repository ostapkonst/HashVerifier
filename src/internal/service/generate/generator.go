// Package generate contains the use-case orchestration for generating checksum files from a directory tree.
package generate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/exclude"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/domain/walk"
)

// GeneratorStatusType marks whether a Generator has begun or completed its run.
type GeneratorStatusType int

const (
	GeneratorStatusFinished GeneratorStatusType = iota
	GeneratorStatusStarted
)

// Generator walks root, hashes non-excluded files with algo, and writes the checksum file. Concurrent; safe to drive from one goroutine.
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
		excludeMatcher:      excludeMatcher,
		status:              GeneratorStatusFinished,
		speedTracker:        result.NewSpeedTracker(),
	}

	return g
}

// Start kicks off the walk and writes the checksum file in a goroutine.
// No-op while running; self-resets on completion, so it is safe to call again.
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

	go g.run()
}

// Wait blocks until the current run completes and returns the terminal error (or nil on success).
func (g *Generator) Wait() error {
	<-g.done
	return <-g.err
}

// MarkWritten classifies err into the Processed / Skipped / WithErrors counters.
func (g *Generator) MarkWritten(err error) {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	switch {
	case err == nil:
		g.stats.Processed++
	case exclude.IsExcludedError(err), walk.IsPathValidationError(err):
		g.stats.Skipped++
	default:
		g.stats.WithErrors++
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

// Results returns the receive-only channel of per-file GenerateResult events for the current run.
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
		g.updateCurrentFileOrStatus("ready to go...")
		g.speedTracker.Reset()
	}()
	defer func() {
		g.rwm.Lock()
		defer g.rwm.Unlock()

		g.status = GeneratorStatusFinished
	}()

	defer close(g.done)
	defer close(g.err)
	defer close(g.resultCh)
	defer g.cancel()

	g.updateCurrentFileOrStatus("forming a list of files for hashing...")

	walkResult, err := walk.WalkDir(g.ctx, g.root, g.followSymbolicLinks, g.sortPaths)
	if err != nil {
		g.err <- fmt.Errorf("walking %s: %w", g.root, err)
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

	for _, file := range files {
		select {
		case <-g.ctx.Done():
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

		if g.excludeMatcher.IsExcluded(relPath) {
			finalPath := filepath.Join(g.dirPrefix, relPath)

			select {
			case g.resultCh <- result.GenerateResult{
				FullPath:  file,
				RelPath:   finalPath,
				Hash:      strings.Repeat("0", algorithm.GetHashLength(g.algo)),
				ReadBytes: 0,
				Err:       exclude.ErrExcludedByUser,
				Status:    result.GenSkipped,
			}:
			case <-g.ctx.Done():
				return
			}

			continue
		}

		hashCalc := hashfn.NewHashCalculator(file, g.algo, g.speedTracker)
		g.currFileHashingProgress.Store(hashCalc.Progress)

		var fileErr error

		hashResult, err := hashCalc.Calculate(g.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				g.err <- fmt.Errorf("hashing %s: %w", file, err)
				return
			}

			fileErr = err
		}

		finalPath := filepath.Join(g.dirPrefix, relPath)

		status := result.GenSuccess

		if fileErr != nil {
			if walk.IsPathValidationError(fileErr) {
				status = result.GenSkipped
			} else {
				status = result.GenFailed
			}
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
			return
		}
	}
}

func filterOutputFile(files []string, outputFile string) ([]string, error) {
	if outputFile == "" {
		return files, nil
	}

	absOutput, err := filepath.Abs(outputFile)
	if err != nil {
		return nil, fmt.Errorf("resolve output file path: %w", err)
	}

	filtered := make([]string, 0, len(files))

	for _, f := range files {
		absF, err := filepath.Abs(f)
		if err != nil {
			return nil, fmt.Errorf("resolve file path %s: %w", f, err)
		}

		if !hashfn.PathsEqual(absF, absOutput) {
			filtered = append(filtered, f)
		}
	}

	return filtered, nil
}

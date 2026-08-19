package generator

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
)

type GeneratorStatusType int

const (
	GeneratorStatusFinished GeneratorStatusType = iota
	GeneratorStatusStarted
)

type Generator struct {
	rwm    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	root                    string
	outputFile              string
	algo                    checksum.Algorithm
	dirPrefix               string
	followSymbolicLinks     bool
	sortPaths               bool
	excludeMatcher          *checksum.ExcludeMatcher
	stats                   checksum.GeneratorStats
	currFileHashingProgress atomic.Value
	speedTracker            *checksum.SpeedTracker

	status GeneratorStatusType

	err      chan error
	resultCh chan checksum.GenerateResult
	done     chan struct{}
}

func NewGeneratorWithExclusions(
	ctx context.Context,
	root string,
	outputFile string,
	algo checksum.Algorithm,
	dirPrefix string,
	followSymbolicLinks bool,
	sortPaths bool,
	excludeMatcher *checksum.ExcludeMatcher,
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
		speedTracker:        checksum.NewSpeedTracker(),
	}

	return g
}

func (g *Generator) Start() {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	if g.status != GeneratorStatusFinished {
		return
	}

	g.resultCh = make(chan checksum.GenerateResult, 1)
	g.done = make(chan struct{})
	g.err = make(chan error, 1)

	g.status = GeneratorStatusStarted

	g.stats = checksum.NewGeneratorStats()
	g.currFileHashingProgress.Store(func() float64 { return 0 })
	g.speedTracker.Reset()

	go g.run()
}

func (g *Generator) Wait() error {
	<-g.done
	return <-g.err
}

func (g *Generator) Stats() checksum.GeneratorStats {
	fileHashProgress := g.currFileHashingProgress.Load().(func() float64)

	g.rwm.RLock()
	defer g.rwm.RUnlock()

	stats := g.stats
	stats.FileHashingProgress = fileHashProgress()
	stats.Speed = g.speedTracker.Speed()

	return stats
}

func (g *Generator) Results() <-chan checksum.GenerateResult {
	return g.resultCh
}

func (g *Generator) updateStats(err error) {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	switch {
	case err == nil:
		g.stats.Processed++
	case checksum.IsExcludedError(err), checksum.IsPathValidationError(err):
		g.stats.Skipped++
	default:
		g.stats.WithErrors++
	}
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

	files, err := checksum.WalkDir(g.ctx, g.root, g.followSymbolicLinks, g.sortPaths)
	if err != nil {
		g.err <- err
		return
	}

	files = filterOutputFile(files, g.outputFile)

	g.updateStatsPending(len(files))

	for _, file := range files {
		g.updateCurrentFileOrStatus(file)
		g.currFileHashingProgress.Store(func() float64 { return 0 })

		relPath, err := filepath.Rel(g.root, file)
		if err != nil {
			g.err <- fmt.Errorf("failed to calculate relative path: %w", err)
			return
		}

		if g.excludeMatcher.IsExcluded(relPath) {
			finalPath := filepath.Join(g.dirPrefix, relPath)

			g.updateStats(checksum.ErrExcludedByUser)

			g.resultCh <- checksum.GenerateResult{
				FullPath:  file,
				RelPath:   finalPath,
				Hash:      strings.Repeat("0", checksum.GetHashLength(g.algo)),
				ReadBytes: 0,
				Err:       checksum.ErrExcludedByUser,
				Status:    checksum.GenSkipped,
			}

			continue
		}

		hashCalc := checksum.NewHashCalculator(file, g.algo, g.speedTracker)
		g.currFileHashingProgress.Store(hashCalc.Progress)

		var fileErr error

		hashResult, err := hashCalc.Calculate(g.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				g.err <- err
				return
			}

			fileErr = err
		}

		finalPath := filepath.Join(g.dirPrefix, relPath)

		status := checksum.GenSuccess

		if fileErr != nil {
			if checksum.IsPathValidationError(fileErr) {
				status = checksum.GenSkipped
			} else {
				status = checksum.GenFailed
			}
		}

		g.updateStats(fileErr)

		g.resultCh <- checksum.GenerateResult{
			FullPath:  file,
			RelPath:   finalPath,
			Hash:      strings.ToLower(hashResult.Hash),
			ReadBytes: hashResult.ReadBytes,
			Err:       fileErr,
			Status:    status,
		}
	}
}

func filterOutputFile(files []string, outputFile string) []string {
	if outputFile == "" {
		return files
	}

	absOutput, err := filepath.Abs(outputFile)
	if err != nil {
		return files
	}

	filtered := make([]string, 0, len(files))

	for _, f := range files {
		absF, err := filepath.Abs(f)
		if err != nil {
			filtered = append(filtered, f)
			continue
		}

		if !checksum.PathsEqual(absF, absOutput) {
			filtered = append(filtered, f)
		}
	}

	return filtered
}

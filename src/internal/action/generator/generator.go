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
	GeneratorStatusStated
)

type Generator struct {
	rwm    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	root                    string
	algo                    checksum.Algorithm
	dirPrefix               string
	followSymbolicLinks     bool
	sortPaths               bool
	stats                   checksum.GeneratorStats
	currFileHashingProgress atomic.Value
	speedTracker            *checksum.SpeedTracker

	status GeneratorStatusType

	err      chan error
	resultCh chan checksum.GenerateResult
	done     chan struct{}
}

func NewGenerator(
	ctx context.Context,
	root string,
	algo checksum.Algorithm,
	dirPrefix string,
	followSymbolicLinks bool,
	sortPaths bool,
) *Generator {
	ctx, cancel := context.WithCancel(ctx)

	g := &Generator{
		ctx:                 ctx,
		cancel:              cancel,
		resultCh:            make(chan checksum.GenerateResult, 1),
		done:                make(chan struct{}),
		err:                 make(chan error, 1),
		root:                root,
		algo:                algo,
		dirPrefix:           dirPrefix,
		followSymbolicLinks: followSymbolicLinks,
		sortPaths:           sortPaths,
		status:              GeneratorStatusFinished,
		speedTracker:        checksum.NewSpeedTracker(),
	}

	g.stats = checksum.NewGeneratorStats()
	g.currFileHashingProgress.Store(func() float64 { return 0 })

	return g
}

func (g *Generator) Start() {
	g.rwm.Lock()
	defer g.rwm.Unlock()

	if g.status != GeneratorStatusFinished {
		return
	}

	g.status = GeneratorStatusStated

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
	case checksum.IsPathValidationError(err):
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

	g.updateStatsPending(len(files))

	for _, file := range files {
		g.updateCurrentFileOrStatus(file)

		hashCalc := checksum.NewHashCalculator(file, g.algo, g.speedTracker)
		g.currFileHashingProgress.Store(hashCalc.Progress)

		var fileErr error

		hastResult, err := hashCalc.Calculate(g.ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				g.err <- err
				return
			}

			fileErr = err
		}

		relPath, err := filepath.Rel(g.root, file)
		if err != nil {
			g.err <- fmt.Errorf("failed to calculate relative path: %w", err)
			return
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
			Hash:      strings.ToLower(hastResult.Hash),
			ReadBytes: hastResult.ReadBytes,
			Err:       fileErr,
			Status:    status,
		}
	}
}

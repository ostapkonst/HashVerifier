// Package verify contains the use-case orchestration for verifying files against an existing checksum file.
package verify

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/parser"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

// VerifierStatusType marks whether a Verifier has begun or completed its run.
type VerifierStatusType int

const (
	VerifierStatusFinished VerifierStatusType = iota
	VerifierStatusStarted
)

// Verifier reads filename's checksum entries, rehashes each one, reports results. Concurrent; safe to drive from one goroutine.
type Verifier struct {
	rwm    sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	filename                string
	stats                   result.VerifierStats
	algo                    algorithm.Algorithm
	currFileHashingProgress atomic.Value
	speedTracker            *result.SpeedTracker

	status VerifierStatusType

	err      chan error
	resultCh chan result.VerifyResult
	done     chan struct{}
}

// NewVerifier constructs a Verifier; Start kicks off the walk, Wait/Results consume it.
func NewVerifier(ctx context.Context, filename string, algo algorithm.Algorithm) *Verifier {
	ctx, cancel := context.WithCancel(ctx)

	v := &Verifier{
		ctx:          ctx,
		cancel:       cancel,
		filename:     filename,
		algo:         algo,
		status:       VerifierStatusFinished,
		speedTracker: result.NewSpeedTracker(),
	}

	return v
}

// Start runs parse/rehash in a goroutine.
// No-op while running; self-resets on completion, so it is safe to call again.
func (v *Verifier) Start() {
	v.rwm.Lock()
	defer v.rwm.Unlock()

	if v.status != VerifierStatusFinished {
		return
	}

	v.resultCh = make(chan result.VerifyResult, 1)
	v.done = make(chan struct{})
	v.err = make(chan error, 1)

	v.status = VerifierStatusStarted

	v.stats = result.NewVerifierStats()
	v.currFileHashingProgress.Store(func() float64 { return 0 })
	v.speedTracker.Reset()

	go v.run()
}

// Wait blocks until the current run completes and returns the terminal error (or nil on success).
func (v *Verifier) Wait() error {
	<-v.done
	return <-v.err
}

// Stats returns the current aggregate progress snapshot including file-hash progress and rolling speed.
func (v *Verifier) Stats() result.VerifierStats {
	fileHashProgress := v.currFileHashingProgress.Load().(func() float64)

	v.rwm.RLock()
	defer v.rwm.RUnlock()

	stats := v.stats
	stats.FileHashingProgress = fileHashProgress()
	stats.Speed = v.speedTracker.Speed()

	return stats
}

// Results returns the receive-only channel of per-file VerifyResult events for the current run.
func (v *Verifier) Results() <-chan result.VerifyResult {
	return v.resultCh
}

// MarkVerified classifies status into the Matched / Mismatch / Unreadable counters; called by the consumer after each per-file comparison.
func (v *Verifier) MarkVerified(status result.VerifyStatusType) {
	v.rwm.Lock()
	defer v.rwm.Unlock()

	switch status {
	case result.HashMatched:
		v.stats.Matched++
	case result.HashMismatch:
		v.stats.Mismatch++
	case result.Unreadable:
		v.stats.Unreadable++
	}
}

func (v *Verifier) updateCurrentFileOrStatus(file string) {
	v.rwm.Lock()
	defer v.rwm.Unlock()

	v.stats.CurrentFileOrStatus = file
}

func (v *Verifier) updateStatsPending(totalFiles int) {
	v.rwm.Lock()
	defer v.rwm.Unlock()

	v.stats.TotalFiles = totalFiles
}

func (v *Verifier) run() {
	defer func() {
		v.updateCurrentFileOrStatus("ready to go...")
		v.speedTracker.Reset()
	}()
	defer func() {
		v.rwm.Lock()
		defer v.rwm.Unlock()

		v.status = VerifierStatusFinished
	}()

	defer close(v.done)
	defer close(v.err)
	defer close(v.resultCh)
	defer v.cancel()

	baseDir := filepath.Dir(v.filename)

	v.updateCurrentFileOrStatus("forming a list of files for verification...")

	checkSum, err := parser.ParseCheckSum(v.ctx, v.filename, v.algo)
	if err != nil {
		v.err <- fmt.Errorf("verifying %s: %w", v.filename, err)
		return
	}

	v.updateStatsPending(len(checkSum))

	for _, line := range checkSum {
		select {
		case <-v.ctx.Done():
			return
		default:
		}

		var path string

		if filepath.IsAbs(line.RelPath) {
			path = line.RelPath
		} else {
			path = filepath.Join(baseDir, line.RelPath)
		}

		v.updateCurrentFileOrStatus(path)

		hashCalc := hashfn.NewHashCalculator(path, v.algo, v.speedTracker)
		v.currFileHashingProgress.Store(hashCalc.Progress)
		hashResult, err := hashCalc.Calculate(v.ctx)

		fileStatus := result.HashMatched

		var fileErr error

		if err != nil {
			if errors.Is(err, context.Canceled) {
				v.err <- err
				return
			}

			fileErr = err
			fileStatus = result.Unreadable
		}

		if fileStatus != result.Unreadable {
			if strings.EqualFold(hashResult.Hash, line.Hash) {
				fileStatus = result.HashMatched
			} else {
				fileStatus = result.HashMismatch
			}
		}

		select {
		case v.resultCh <- result.VerifyResult{
			Path:         line.RelPath,
			FullPath:     path,
			ExpectedHash: strings.ToLower(line.Hash),
			ActualHash:   strings.ToLower(hashResult.Hash),
			ReadBytes:    hashResult.ReadBytes,
			Status:       fileStatus,
			Err:          fileErr,
		}:
		case <-v.ctx.Done():
			return
		}
	}
}

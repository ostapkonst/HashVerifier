// Package verify contains the use-case orchestration for verifying files against an existing checksum file.
package verify

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/parser"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"github.com/ostapkonst/HashVerifier/internal/platform/errs"
)

// VerifierStatusType marks whether a Verifier has begun or completed its run.
type VerifierStatusType int

// VerifierStatusType values: Finished is both the initial and terminal state, Started means a run is in flight.
const (
	VerifierStatusFinished VerifierStatusType = iota
	VerifierStatusStarted
)

// Verifier reads filename's checksum entries, rehashes each one, and reports results.
// Stats and MarkVerified are mutex-protected; Wait/Results only read channels installed by Start,
// so call them from the goroutine that started the run.
//
// A Verifier instance is intended for a single run: the internal ctx is canceled when the run
// completes, and a second Start would observe an already-canceled context. Callers needing multiple
// runs should construct a fresh Verifier per call (this matches what every service entry point does today).
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

// NewVerifier constructs a Verifier; Start kicks off parse/rehash, Wait/Results consume it.
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

// Start launches the parse-and-rehash goroutine; a no-op while a run is in flight.
// A Verifier is single-use: run cancels the internal ctx, so a second Start fails at once with that ctx error.
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
	// Every early return must leave a terminal error in v.err; on cancellation the
	// loop exits via different select sites, so each cancel exit sends ctx.Err() itself.
	// The channel is buffered (cap 1) and only this goroutine ever sends the first one.

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
			v.err <- fmt.Errorf("verifying %s: %w", v.filename, v.ctx.Err())
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
			if errs.IsContextDone(err) {
				v.err <- fmt.Errorf("hashing %s: %w", path, err)
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
			v.err <- fmt.Errorf("verifying %s: %w", v.filename, v.ctx.Err())
			return
		}
	}
}

package verify

import (
	"context"
	"errors"
	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/hashfn"
	"github.com/ostapkonst/HashVerifier/internal/domain/parser"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

type VerifierStatusType int

const (
	VerifierStatusFinished VerifierStatusType = iota
	VerifierStatusStarted
)

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

func (v *Verifier) Wait() error {
	<-v.done
	return <-v.err
}

func (v *Verifier) Stats() result.VerifierStats {
	fileHashingProgress := v.currFileHashingProgress.Load().(func() float64)

	v.rwm.RLock()
	defer v.rwm.RUnlock()

	stats := v.stats
	stats.FileHashingProgress = fileHashingProgress()
	stats.Speed = v.speedTracker.Speed()

	return stats
}

func (v *Verifier) Results() <-chan result.VerifyResult {
	return v.resultCh
}

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
		v.err <- err
		return
	}

	v.updateStatsPending(len(checkSum))

	for _, line := range checkSum {
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

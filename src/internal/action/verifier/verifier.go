package verifier

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/checksum"
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
	stats                   checksum.VerifierStats
	algo                    checksum.Algorithm
	currFileHashingProgress atomic.Value
	speedTracker            *checksum.SpeedTracker

	status VerifierStatusType

	err      chan error
	resultCh chan checksum.VerifyResult
	done     chan struct{}
}

func NewVerifier(ctx context.Context, filename string, algo checksum.Algorithm) *Verifier {
	ctx, cancel := context.WithCancel(ctx)

	v := &Verifier{
		ctx:          ctx,
		cancel:       cancel,
		filename:     filename,
		algo:         algo,
		status:       VerifierStatusFinished,
		speedTracker: checksum.NewSpeedTracker(),
	}

	return v
}

func (v *Verifier) Start() {
	v.rwm.Lock()
	defer v.rwm.Unlock()

	if v.status != VerifierStatusFinished {
		return
	}

	v.resultCh = make(chan checksum.VerifyResult, 1)
	v.done = make(chan struct{})
	v.err = make(chan error, 1)

	v.status = VerifierStatusStarted

	v.stats = checksum.NewVerifierStats()
	v.currFileHashingProgress.Store(func() float64 { return 0 })
	v.speedTracker.Reset()

	go v.run()
}

func (v *Verifier) Wait() error {
	<-v.done
	return <-v.err
}

func (v *Verifier) Stats() checksum.VerifierStats {
	fileHashingProgress := v.currFileHashingProgress.Load().(func() float64)

	v.rwm.RLock()
	defer v.rwm.RUnlock()

	stats := v.stats
	stats.FileHashingProgress = fileHashingProgress()
	stats.Speed = v.speedTracker.Speed()

	return stats
}

func (v *Verifier) Results() <-chan checksum.VerifyResult {
	return v.resultCh
}

func (v *Verifier) MarkVerified(status checksum.VerifyStatusType) {
	v.rwm.Lock()
	defer v.rwm.Unlock()

	switch status {
	case checksum.HashMatched:
		v.stats.Matched++
	case checksum.HashMismatch:
		v.stats.Mismatch++
	case checksum.Unreadable:
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

	checkSum, err := checksum.ParseCheckSum(v.ctx, v.filename, v.algo)
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

		hashCalc := checksum.NewHashCalculator(path, v.algo, v.speedTracker)
		v.currFileHashingProgress.Store(hashCalc.Progress)
		hashResult, err := hashCalc.Calculate(v.ctx)

		fileStatus := checksum.HashMatched

		var fileErr error

		if err != nil {
			if errors.Is(err, context.Canceled) {
				v.err <- err
				return
			}

			fileErr = err
			fileStatus = checksum.Unreadable
		}

		if fileStatus != checksum.Unreadable {
			if strings.EqualFold(hashResult.Hash, line.Hash) {
				fileStatus = checksum.HashMatched
			} else {
				fileStatus = checksum.HashMismatch
			}
		}

		select {
		case v.resultCh <- checksum.VerifyResult{
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

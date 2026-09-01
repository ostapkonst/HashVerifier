package hashfn

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

// MultiHashResult holds per-algorithm hex digests after one streaming pass.
type MultiHashResult struct {
	ReadBytes int64
	Hashes    map[algorithm.Algorithm]string
}

// MultiHashCalculator streams a file through several hash.Hash algorithms in a single read pass.
type MultiHashCalculator struct {
	algorithms     []algorithm.Algorithm
	path           string
	fileSize       int64
	readBytes      atomic.Int64
	readAllContent atomic.Bool
	speedTracker   *result.SpeedTracker
}

// NewMultiHashCalculator wires the calculator to path and algorithms; speedTracker must be non-nil (nil
// panics on the first read) and receives per-read byte counts for throughput display.
func NewMultiHashCalculator(path string, algorithms []algorithm.Algorithm, speedTracker *result.SpeedTracker) *MultiHashCalculator {
	return &MultiHashCalculator{
		algorithms:     algorithms,
		path:           path,
		fileSize:       calculateFileSize(path),
		readAllContent: atomic.Bool{},
		speedTracker:   speedTracker,
	}
}

// Progress returns the read-bytes-over-file-size ratio in [0, 1]; reads 1.0 once readAllContent is set (after Calculate completes).
func (c *MultiHashCalculator) Progress() float64 {
	if c.readAllContent.Load() {
		return 1
	}

	if c.fileSize == 0 {
		return 0
	}

	readBytes := c.readBytes.Load()

	if readBytes >= c.fileSize {
		return 1
	}

	return float64(readBytes) / float64(c.fileSize)
}

// Calculate streams the file through all configured algorithms in one pass (empty Hashes when none).
func (c *MultiHashCalculator) Calculate(ctx context.Context) (result MultiHashResult, err error) {
	c.readAllContent.Store(false)
	c.readBytes.Store(0)

	canceled := false

	defer func() {
		if !canceled {
			c.readAllContent.Store(true)
		}
	}()

	result = MultiHashResult{
		Hashes: make(map[algorithm.Algorithm]string, len(c.algorithms)),
	}

	select {
	case <-ctx.Done():
		canceled = true
		return result, fmt.Errorf("hash %s: %w", c.path, ctx.Err())
	default:
	}

	if len(c.algorithms) == 0 {
		return result, nil
	}

	// Non-regular files (FIFOs, sockets, devices) must not be opened: the open/read of a FIFO waits for a
	// writer and cannot be interrupted by context cancellation, freezing generate/verify/hash.
	if err = ensureRegularFile(c.path); err != nil {
		return result, err
	}

	f, err := os.Open(c.path)
	if err != nil {
		// fs.PathError already embeds the op ("open") and path; another prefix would double them.
		return result, err
	}

	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", c.path, cerr)
		}
	}()

	hashers := make(map[algorithm.Algorithm]hash.Hash, len(c.algorithms))
	for _, algoType := range c.algorithms {
		hashers[algoType] = algorithm.NewHasher(algoType)
	}

	buf := make([]byte, HashBufferSize)

	for {
		select {
		case <-ctx.Done():
			canceled = true
			return result, fmt.Errorf("hash %s: %w", c.path, ctx.Err())
		default:
		}

		n, err := f.Read(buf)
		if n > 0 {
			result.ReadBytes += int64(n)
			c.readBytes.Store(result.ReadBytes)
			c.speedTracker.AddBytes(int64(n))

			for _, h := range hashers {
				if _, werr := h.Write(buf[:n]); werr != nil {
					return result, fmt.Errorf("write %s: %w", c.path, werr)
				}
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			// fs.PathError already embeds the op ("read") and path; another prefix would double them.
			return result, err
		}
	}

	c.readAllContent.Store(true)

	for algoType, h := range hashers {
		result.Hashes[algoType] = hex.EncodeToString(h.Sum(nil))
	}

	return result, nil
}

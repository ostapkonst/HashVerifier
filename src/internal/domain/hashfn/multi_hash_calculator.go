package hashfn

import (
	"context"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

type MultiHashResult struct {
	ReadBytes int64
	Hashes    map[algorithm.Algorithm]string
}

type MultiHashCalculator struct {
	algorithms     []algorithm.Algorithm
	path           string
	fileSize       int64
	readBytes      atomic.Int64
	readAllContent atomic.Bool
	speedTracker   *result.SpeedTracker
}

func NewMultiHashCalculator(path string, algorithms []algorithm.Algorithm, speedTracker *result.SpeedTracker) *MultiHashCalculator {
	return &MultiHashCalculator{
		algorithms:     algorithms,
		path:           path,
		fileSize:       calculateFileSize(path),
		readAllContent: atomic.Bool{},
		speedTracker:   speedTracker,
	}
}

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

func (c *MultiHashCalculator) Calculate(ctx context.Context) (MultiHashResult, error) {
	c.readAllContent.Store(false)
	c.readBytes.Store(0)

	canceled := false

	defer func() {
		if !canceled {
			c.readAllContent.Store(true)
		}
	}()

	result := MultiHashResult{
		Hashes: make(map[algorithm.Algorithm]string, len(c.algorithms)),
	}

	select {
	case <-ctx.Done():
		canceled = true
		return result, ctx.Err()
	default:
	}

	if len(c.algorithms) == 0 {
		return result, nil
	}

	f, err := os.Open(c.path)
	if err != nil {
		return result, err
	}

	defer f.Close() //nolint:errcheck

	hashers := make(map[algorithm.Algorithm]hash.Hash, len(c.algorithms))
	for _, algoType := range c.algorithms {
		hashers[algoType] = algorithm.NewHasher(algoType)
	}

	buf := make([]byte, HashBufferSize)

	for {
		select {
		case <-ctx.Done():
			canceled = true
			return result, ctx.Err()
		default:
		}

		n, err := f.Read(buf)
		if n > 0 {
			result.ReadBytes += int64(n)
			c.readBytes.Store(result.ReadBytes)
			c.speedTracker.AddBytes(int64(n))

			for _, h := range hashers {
				if _, werr := h.Write(buf[:n]); werr != nil {
					return result, werr
				}
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return result, err
		}
	}

	c.readAllContent.Store(true)

	for algoType, h := range hashers {
		result.Hashes[algoType] = hex.EncodeToString(h.Sum(nil))
	}

	return result, nil
}

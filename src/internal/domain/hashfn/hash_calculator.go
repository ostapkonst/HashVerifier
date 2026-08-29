// Package hashfn provides streaming hasher calculators that read files once and produce hex digests.
package hashfn

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

// HashBufferSize is the per-Read chunk size for streaming a file through the hasher.
const HashBufferSize = 128 * 1024

// HashResult carries a file's digest plus the byte count hashed (used by the progress reporter).
type HashResult struct {
	ReadBytes int64
	Hash      string
}

// Sentinel errors for paths that cannot be represented unambiguously in a checksum file.
var (
	ErrPathContainsInvalidSeparator = errors.New("backslash in path (not supported)")
	ErrPathContainsNewline          = errors.New("newline in path (not supported)")
	ErrPathContainsCarriageReturn   = errors.New("carriage return in path (not supported)")
	ErrCRC32PathStartsWithSemicolon = errors.New("path starts with semicolon (not supported by SFV format)")
	ErrCRC32PathEndWithSpace        = errors.New("path ends with space (not supported by SFV format)")
)

// ErrNotARegularFile is returned when a path resolves to a non-regular file type (FIFO, socket, device);
// such files are refused before os.Open because reading them can block indefinitely.
var ErrNotARegularFile = errors.New("not a regular file")

// HashCalculator streams a file through one hash.Hash and reports progress; each Calculate call resets its
// progress state, so a single instance can hash repeatedly.
type HashCalculator struct {
	algoType       algorithm.Algorithm
	path           string
	fileSize       int64
	readBytes      atomic.Int64
	readAllContent atomic.Bool
	speedTracker   *result.SpeedTracker
}

// NewHashCalculator wires a single-algorithm streamer to path; speedTracker must be non-nil (nil panics on
// the first read) and receives per-read byte counts for throughput display.
func NewHashCalculator(path string, algoType algorithm.Algorithm, speedTracker *result.SpeedTracker) *HashCalculator {
	return &HashCalculator{
		algoType:       algoType,
		path:           path,
		fileSize:       calculateFileSize(path),
		readAllContent: atomic.Bool{},
		speedTracker:   speedTracker,
	}
}

// Progress returns the read-bytes-over-file-size ratio in [0, 1]; reads 1.0 once readAllContent is set (after Calculate completes).
func (c *HashCalculator) Progress() float64 {
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

// Calculate streams the configured file through the algorithm; honors ctx cancellation and updates progress as bytes are read.
func (c *HashCalculator) Calculate(ctx context.Context) (HashResult, error) {
	c.readAllContent.Store(false)
	c.readBytes.Store(0)

	canceled := false
	defer func() {
		if !canceled {
			c.readAllContent.Store(true)
		}
	}()

	result := HashResult{
		Hash: strings.Repeat("0", algorithm.GetHashLength(c.algoType)),
	}

	select {
	case <-ctx.Done():
		canceled = true
		return result, fmt.Errorf("hash %s: %w", c.path, ctx.Err())
	default:
	}

	switch {
	case os.PathSeparator == '/' && strings.Contains(c.path, "\\"):
		return result, ErrPathContainsInvalidSeparator
	case strings.Contains(c.path, "\n"):
		return result, ErrPathContainsNewline
	case strings.Contains(c.path, "\r"):
		return result, ErrPathContainsCarriageReturn
	case c.algoType == algorithm.CRC32 && strings.HasPrefix(c.path, ";"):
		return result, ErrCRC32PathStartsWithSemicolon
	case c.algoType == algorithm.CRC32 && strings.HasSuffix(c.path, " "):
		return result, ErrCRC32PathEndWithSpace
	}

	// Non-regular files (FIFOs, sockets, devices) must not be opened: the open/read of a FIFO waits for a
	// writer and cannot be interrupted by context cancellation, freezing generate/verify/hash.
	if err := ensureRegularFile(c.path); err != nil {
		return result, err
	}

	f, err := os.Open(c.path)
	if err != nil {
		return result, fmt.Errorf("open %s: %w", c.path, err)
	}
	defer f.Close() //nolint:errcheck

	h := algorithm.NewHasher(c.algoType)
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

			if _, werr := h.Write(buf[:n]); werr != nil {
				return result, fmt.Errorf("write %s: %w", c.path, werr)
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return result, fmt.Errorf("read %s: %w", c.path, err)
		}
	}

	c.readAllContent.Store(true)

	result.Hash = hex.EncodeToString(h.Sum(nil))

	return result, nil
}

// ensureRegularFile refuses non-regular paths (FIFOs, sockets, devices): their open/read can block forever
// and cannot be interrupted by context cancellation, freezing generate/verify/hash.
func ensureRegularFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf("hash %s: %w", path, ErrNotARegularFile)
	}

	return nil
}

func calculateFileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return fi.Size()
}

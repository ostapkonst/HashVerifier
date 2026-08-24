package hashfn

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/ostapkonst/HashVerifier/internal/domain/algorithm"
	"github.com/ostapkonst/HashVerifier/internal/domain/result"
)

const HashBufferSize = 128 * 1024 // 128KB

type HashResult struct {
	ReadBytes int64
	Hash      string
}

var (
	ErrPathContainsInvalidSeparator = fmt.Errorf("backslash in path (not supported)")
	ErrPathContainsNewline          = fmt.Errorf("newline in path (not supported)")
	ErrPathContainsCarriageReturn   = fmt.Errorf("carriage return in path (not supported)")
	ErrCRC32PathStartsWithSemicolon = fmt.Errorf("path starts with semicolon (not supported by SFV format)")
	ErrCRC32PathEndWithSpace        = fmt.Errorf("path ends with space (not supported by SFV format)")
)

type HashCalculator struct {
	algoType       algorithm.Algorithm
	path           string
	fileSize       int64
	readBytes      atomic.Int64
	readAllContent atomic.Bool
	speedTracker   *result.SpeedTracker
}

func NewHashCalculator(path string, algoType algorithm.Algorithm, speedTracker *result.SpeedTracker) *HashCalculator {
	return &HashCalculator{
		algoType:       algoType,
		path:           path,
		fileSize:       calculateFileSize(path),
		readAllContent: atomic.Bool{},
		speedTracker:   speedTracker,
	}
}

func (c *HashCalculator) Progress() float64 {
	if c.readAllContent.Load() {
		return 1 // если прочитали все, то прогресс 100%, даже если планировали, что файл окажется больше
	}

	if c.fileSize == 0 {
		return 0 // если не знаем размер, то прогресс 0 т. к. можем читать сколь угодно долго
	}

	readBytes := c.readBytes.Load()

	if readBytes >= c.fileSize { // можем считать больше, чем планировали
		return 1
	}

	return float64(readBytes) / float64(c.fileSize)
}

func (c *HashCalculator) Calculate(ctx context.Context) (HashResult, error) {
	c.readAllContent.Store(false)
	c.readBytes.Store(0)

	canceled := false

	defer func() {
		if !canceled {
			c.readAllContent.Store(true) // если не отмена, то считаем, что прогресс 100%
		}
	}()

	result := HashResult{
		Hash: strings.Repeat("0", algorithm.GetHashLength(c.algoType)), // заглушка, чтобы не было пустого хеша при ошибке
	}

	select {
	case <-ctx.Done():
		canceled = true
		return result, ctx.Err()
	default:
	}

	switch {
	case os.PathSeparator == '/' && strings.Contains(c.path, "\\"):
		return result, ErrPathContainsInvalidSeparator // пришлось добавить ограничение на виндовые пути
	case strings.Contains(c.path, "\n"):
		return result, ErrPathContainsNewline // имя с переносом строки невозможно корректно сохранить в checksum-файле
	case strings.Contains(c.path, "\r"):
		return result, ErrPathContainsCarriageReturn // символ возврата каретки может быть удалён при чтении checksum-файла
	case c.algoType == algorithm.CRC32 && strings.HasPrefix(c.path, ";"):
		return result, ErrCRC32PathStartsWithSemicolon
	case c.algoType == algorithm.CRC32 && strings.HasSuffix(c.path, " "):
		return result, ErrCRC32PathEndWithSpace
	}

	f, err := os.Open(c.path)
	if err != nil {
		return result, err
	}

	defer f.Close() //nolint:errcheck

	h := algorithm.NewHasher(c.algoType)
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

			if _, werr := h.Write(buf[:n]); werr != nil {
				return result, werr
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

	result.Hash = hex.EncodeToString(h.Sum(nil))

	return result, nil
}

func calculateFileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}

	return fi.Size()
}

// Package algorithm identifies hash algorithms by file extension and produces hashers.
package algorithm

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"path/filepath"
	"strings"

	"github.com/zeebo/xxh3"
	"golang.org/x/crypto/md4" //nolint:staticcheck
	"lukechampine.com/blake3"
)

// Algorithm enumerates supported hash algorithms with value zero meaning unknown.
type Algorithm int

const (
	Unknown Algorithm = iota
	MD4
	MD5
	SHA1
	CRC32
	SHA256
	SHA384
	SHA512
	SHA3_256
	SHA3_384
	SHA3_512
	BLAKE3
	XXH3
	XXH128
)

// SupportedAlgorithms lists every Algorithm the app can hash and verify.
var SupportedAlgorithms = []Algorithm{
	CRC32, MD4, MD5, SHA1,
	SHA256, SHA384, SHA512,
	SHA3_256, SHA3_384, SHA3_512,
	BLAKE3, XXH3, XXH128,
}

// ErrAlgorithmNotSpecified is returned when no algorithm could be resolved for the operation.
var ErrAlgorithmNotSpecified = errors.New("algorithm not specified")

func (a Algorithm) String() string {
	switch a {
	case MD4:
		return "md4"
	case MD5:
		return "md5"
	case SHA1:
		return "sha1"
	case CRC32:
		return "crc32"
	case SHA256:
		return "sha256"
	case SHA384:
		return "sha384"
	case SHA512:
		return "sha512"
	case SHA3_256:
		return "sha3-256"
	case SHA3_384:
		return "sha3-384"
	case SHA3_512:
		return "sha3-512"
	case BLAKE3:
		return "blake3"
	case XXH3:
		return "xxh3"
	case XXH128:
		return "xxh128"
	default:
		return "unknown"
	}
}

// DisplayName returns a human-readable label for GUI elements.
func (a Algorithm) DisplayName() string {
	switch a {
	case MD4:
		return "MD4"
	case MD5:
		return "MD5"
	case SHA1:
		return "SHA-1"
	case CRC32:
		return "CRC-32"
	case SHA256:
		return "SHA-256"
	case SHA384:
		return "SHA-384"
	case SHA512:
		return "SHA-512"
	case SHA3_256:
		return "SHA3-256"
	case SHA3_384:
		return "SHA3-384"
	case SHA3_512:
		return "SHA3-512"
	case BLAKE3:
		return "BLAKE3"
	case XXH3:
		return "XXH3"
	case XXH128:
		return "XXH128"
	default:
		return "Unknown"
	}
}

// Extension returns the canonical file extension including the leading dot (e.g. ".sha256"). Panics for Unknown.
func (a Algorithm) Extension() string {
	if a == CRC32 {
		return ".sfv"
	}

	if a == Unknown {
		panic("failed to get extension for unknown algorithm")
	}

	return "." + a.String()
}

// AlgorithmFromExtension resolves filename's extension (with or without leading dot, case-insensitive) to an Algorithm.
func AlgorithmFromExtension(filename string) (Algorithm, error) {
	switch ext := strings.ToLower(filepath.Ext(filename)); ext {
	case ".md4":
		return MD4, nil
	case ".md5":
		return MD5, nil
	case ".sha1":
		return SHA1, nil
	case ".sfv":
		return CRC32, nil
	case ".sha256":
		return SHA256, nil
	case ".sha384":
		return SHA384, nil
	case ".sha512":
		return SHA512, nil
	case ".sha3-256":
		return SHA3_256, nil
	case ".sha3-384":
		return SHA3_384, nil
	case ".sha3-512":
		return SHA3_512, nil
	case ".blake3":
		return BLAKE3, nil
	case ".xxh3":
		return XXH3, nil
	case ".xxh128":
		return XXH128, nil
	default:
		return Unknown, fmt.Errorf("unsupported extension: %s", ext)
	}
}

// ResolveAlgorithm prefers the hint string (e.g. ".sha256") when non-empty, otherwise falls back to ResolveAlgorithmFromFile.
func ResolveAlgorithm(hint, file string) (Algorithm, error) {
	if hint != "" {
		return AlgorithmFromExtension(hint)
	}

	return ResolveAlgorithmFromFile(file)
}

// ResolveAlgorithmFromFile detects Algorithm via AlgorithmFromAllSumsFiles first, falling back to AlgorithmFromExtension.
func ResolveAlgorithmFromFile(file string) (Algorithm, error) {
	if a, err := AlgorithmFromAllSumsFiles(file); err == nil {
		return a, nil
	}

	return AlgorithmFromExtension(file)
}

// GetHashLength returns the hex character count for algo's digest (e.g. 64 for SHA-256). Panics for unsupported values.
func GetHashLength(algo Algorithm) int {
	switch algo {
	case MD4:
		return 32
	case MD5:
		return 32
	case SHA1:
		return 40
	case CRC32:
		return 8
	case SHA256:
		return 64
	case SHA384:
		return 96
	case SHA512:
		return 128
	case SHA3_256:
		return 64
	case SHA3_384:
		return 96
	case SHA3_512:
		return 128
	case BLAKE3:
		return 64
	case XXH3:
		return 16
	case XXH128:
		return 32
	default:
		panic("unsupported algorithm")
	}
}

// NewHasher returns a fresh hash.Hash for algo; panics on Unknown (callers gate on Algorithm validity first).
func NewHasher(algo Algorithm) hash.Hash {
	switch algo {
	case MD4:
		return md4.New()
	case MD5:
		return md5.New()
	case SHA1:
		return sha1.New()
	case CRC32:
		return crc32.NewIEEE()
	case SHA256:
		return sha256.New()
	case SHA384:
		return sha512.New384()
	case SHA512:
		return sha512.New()
	case SHA3_256:
		return sha3.New256()
	case SHA3_384:
		return sha3.New384()
	case SHA3_512:
		return sha3.New512()
	case BLAKE3:
		return blake3.New(32, nil)
	case XXH3:
		return xxh3.New()
	case XXH128:
		return xxh3.New128()
	default:
		panic("unsupported algorithm")
	}
}

// IsValidHashLength reports whether hash's hex length matches algo's expected digest width.
func IsValidHashLength(hash string, algo Algorithm) bool {
	return len(hash) == GetHashLength(algo)
}

// AlgorithmFromAllSumsFiles resolves SUMS-style filenames (coreutils convention) to an Algorithm.
func AlgorithmFromAllSumsFiles(path string) (Algorithm, error) {
	allSuffixes := []string{"SUMS", "SUM", "SUMS.TXT", "SUM.TXT"}

	for _, suffix := range allSuffixes {
		a, err := algorithmFromSumsFile(path, suffix)
		if err == nil {
			return a, nil
		}
	}

	return Unknown, fmt.Errorf("not a SUMS file")
}

// IsCanonicalAlgorithm reports whether s is the canonical extension (with leading dot) of any supported algorithm.
func IsCanonicalAlgorithm(s string) bool {
	for _, a := range SupportedAlgorithms {
		if a.Extension() == s {
			return true
		}
	}

	return false
}

func algorithmFromSumsFile(path, suffix string) (Algorithm, error) {
	upperSuffix := strings.ToUpper(suffix)

	base := strings.ToUpper(filepath.Base(path))
	if !strings.HasSuffix(base, upperSuffix) {
		return Unknown, fmt.Errorf("not a SUMS file")
	}

	prefix := strings.TrimSuffix(base, upperSuffix)
	ext := "." + prefix

	return AlgorithmFromExtension(ext)
}

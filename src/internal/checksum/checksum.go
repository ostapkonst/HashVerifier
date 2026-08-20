package checksum

import (
	"context"

	"github.com/ostapkonst/HashVerifier/internal/checksum/algo"
	"github.com/ostapkonst/HashVerifier/internal/checksum/calculator"
	"github.com/ostapkonst/HashVerifier/internal/checksum/exclude"
	"github.com/ostapkonst/HashVerifier/internal/checksum/parser"
	"github.com/ostapkonst/HashVerifier/internal/checksum/stats"
)

type Algorithm = algo.Algorithm

type VerifyStatusType = stats.VerifyStatusType

const (
	HashMatched  = stats.HashMatched
	HashMismatch = stats.HashMismatch
	Unreadable   = stats.Unreadable
)

type (
	GenerateResult     = stats.GenerateResult
	GenerateStatusType = stats.GenerateStatusType
	VerifyResult       = stats.VerifyResult
)

const (
	GenSuccess = stats.GenSuccess
	GenSkipped = stats.GenSkipped
	GenFailed  = stats.GenFailed
)

type (
	GeneratorStats = stats.GeneratorStats
	VerifierStats  = stats.VerifierStats
	SpeedTracker   = stats.SpeedTracker
)

func AlgorithmFromExtension(filename string) (Algorithm, error) {
	return algo.AlgorithmFromExtension(filename)
}

func ResolveAlgorithm(hint, file string) (Algorithm, error) {
	return algo.ResolveAlgorithm(hint, file)
}

const Unknown = algo.Unknown

var SupportedAlgorithms = algo.SupportedAlgorithms

func ParseCheckSum(ctx context.Context, filename string, a Algorithm) ([]parser.CheckSumLine, error) {
	return parser.ParseCheckSum(ctx, filename, a)
}

func NewHashCalculator(path string, a Algorithm, st *SpeedTracker) *calculator.HashCalculator {
	return calculator.NewHashCalculator(path, a, st)
}

func NewMultiHashCalculator(path string, algorithms []Algorithm, st *SpeedTracker) *calculator.MultiHashCalculator {
	return calculator.NewMultiHashCalculator(path, algorithms, st)
}

func WalkDir(ctx context.Context, path string, followSymbolicLinks, sortPaths bool) ([]string, error) {
	return calculator.WalkDir(ctx, path, followSymbolicLinks, sortPaths)
}

func GetPrefixForFilesInChecksum(folder, file string) (string, error) {
	return calculator.GetPrefixForFilesInChecksum(folder, file)
}

func PathsEqual(a, b string) bool {
	return calculator.PathsEqual(a, b)
}

func FormatLine(relPath, hashStr string, a Algorithm) string {
	return calculator.FormatLine(relPath, hashStr, a)
}

func IsPathValidationError(err error) bool {
	return calculator.IsPathValidationError(err)
}

func NewSpeedTracker() *SpeedTracker {
	return stats.NewSpeedTracker()
}

func NewGeneratorStats() GeneratorStats {
	return stats.NewGeneratorStats()
}

func NewVerifierStats() VerifierStats {
	return stats.NewVerifierStats()
}

func AlgorithmFromAllSumsFiles(path string) (Algorithm, error) {
	return algo.AlgorithmFromAllSumsFiles(path)
}

func IsCanonicalAlgorithm(s string) bool {
	return algo.IsCanonicalAlgorithm(s)
}

func GetHashLength(a Algorithm) int {
	return algo.GetHashLength(a)
}

type ExcludeMatcher = exclude.Matcher

func NewExcludeMatcher(relPaths []string) *ExcludeMatcher {
	return exclude.NewMatcher(relPaths)
}

func IsExcludedError(err error) bool {
	return exclude.IsExcludedError(err)
}

var ErrExcludedByUser = exclude.ErrExcludedByUser

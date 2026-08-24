// Package result defines status enums, per-file result records, and progress statistics shared by generator/verifier services.
package result

type VerifyStatusType int

const (
	HashMatched VerifyStatusType = iota
	HashMismatch
	Unreadable
)

// String returns the canonical uppercase label rendered in logs and tree views.
func (v VerifyStatusType) String() string {
	switch v {
	case HashMatched:
		return "MATCHED"
	case HashMismatch:
		return "MISMATCH"
	case Unreadable:
		return "UNREADABLE"
	default:
		panic("unknown status")
	}
}

// Priority returns the sort key (lower = higher priority) used by the GUI to order rows.
func (v VerifyStatusType) Priority() int {
	switch v {
	case HashMatched:
		return 0
	case Unreadable:
		return 1
	case HashMismatch:
		return 2
	default:
		return 3
	}
}

// Color returns the Pango/GTK color name associated with this status (e.g. "firebrick1" for mismatch).
func (v VerifyStatusType) Color() string {
	switch v {
	case HashMatched:
		return "green"
	case Unreadable:
		return "dark orange"
	case HashMismatch:
		return "firebrick1"
	default:
		return ""
	}
}

// GenerateStatusType classifies the per-file outcome of the generator.
type GenerateStatusType int

const (
	GenSuccess GenerateStatusType = iota
	GenSkipped
	GenFailed
)

func (g GenerateStatusType) String() string {
	switch g {
	case GenSuccess:
		return "SUCCESS"
	case GenSkipped:
		return "SKIPPED"
	case GenFailed:
		return "FAILED"
	default:
		panic("unknown status")
	}
}

// Priority returns the sort key (lower = higher priority) used by the GUI to order rows.
func (g GenerateStatusType) Priority() int {
	switch g {
	case GenSuccess:
		return 0
	case GenSkipped:
		return 1
	case GenFailed:
		return 2
	default:
		return 3
	}
}

// Color returns the Pango/GTK color name associated with this status.
func (g GenerateStatusType) Color() string {
	switch g {
	case GenSuccess:
		return "green"
	case GenSkipped:
		return "gray"
	case GenFailed:
		return "firebrick1"
	default:
		return ""
	}
}

// VerifyResult is emitted by the verifier service for each file processed.
type VerifyResult struct {
	Path         string
	FullPath     string
	ActualHash   string
	ExpectedHash string
	Status       VerifyStatusType
	ReadBytes    int64
	Err          error
}

// GenerateResult is emitted by the generator service for each file processed.
type GenerateResult struct {
	RelPath   string
	FullPath  string
	Hash      string
	ReadBytes int64
	Err       error
	Status    GenerateStatusType
}

// GeneratorStats is the aggregate progress state for a generate run.
type GeneratorStats struct {
	TotalFiles          int
	Processed           int
	WithErrors          int
	Skipped             int
	CurrentFileOrStatus string
	FileHashingProgress float64
	Speed               float64
}

// NewGeneratorStats returns stats initialised with a "ready" placeholder.
func NewGeneratorStats() GeneratorStats {
	return GeneratorStats{
		CurrentFileOrStatus: "ready to go...",
	}
}

// Pending is the number of files not yet processed, skipped, or errored.
func (g GeneratorStats) Pending() int {
	return g.TotalFiles - g.Processed - g.WithErrors - g.Skipped
}

// TotalProgress returns 0..1 fraction of files no longer pending.
func (g GeneratorStats) TotalProgress() float64 {
	if g.TotalFiles == 0 {
		return 0
	}

	return float64(g.TotalFiles-g.Pending()) / float64(g.TotalFiles)
}

// VerifierStats is the aggregate progress state for a verify run.
type VerifierStats struct {
	TotalFiles          int
	Matched             int
	Mismatch            int
	Unreadable          int
	CurrentFileOrStatus string
	FileHashingProgress float64
	Speed               float64
}

// NewVerifierStats returns stats initialised with a "ready" placeholder.
func NewVerifierStats() VerifierStats {
	return VerifierStats{
		CurrentFileOrStatus: "ready to go...",
	}
}

// Pending is the number of files not yet matched, mismatched, or unreadable.
func (v VerifierStats) Pending() int { return v.TotalFiles - v.Matched - v.Mismatch - v.Unreadable }

// TotalProgress returns 0..1 fraction of files no longer pending.
func (v VerifierStats) TotalProgress() float64 {
	if v.TotalFiles == 0 {
		return 0
	}

	return float64(v.TotalFiles-v.Pending()) / float64(v.TotalFiles)
}

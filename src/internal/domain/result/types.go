// Package result defines status enums, per-file result records, and progress statistics
// shared by the generator, verifier, and discoverer services.
package result

import "time"

// ProgressUpdateInterval is how often the streaming services publish a progress update.
// 50ms keeps the live counters smooth without flooding the GTK main loop with idle callbacks.
const ProgressUpdateInterval = 50 * time.Millisecond

// VerifyResult is one file's verify outcome — emitted as the verifier streams so the
// GUI can render rows as they complete.
type VerifyResult struct {
	Path         string
	FullPath     string
	ActualHash   string
	ExpectedHash string
	Status       VerifyStatusType
	ReadBytes    int64
	Err          error
}

// GenerateResult is one file's generate outcome — emitted as the generator streams so the
// GUI can render rows as they complete.
type GenerateResult struct {
	RelPath   string
	FullPath  string
	Hash      string
	ReadBytes int64
	Err       error
	Status    GenerateStatusType
}

// DiscoverResult is one file's discover outcome — emitted as the discoverer streams so the
// GUI can render rows as they complete. Path is the absolute path shown to the user.
type DiscoverResult struct {
	Path         string
	ActualHash   string
	EmbeddedHash string
	Status       DiscoverStatusType
	ReadBytes    int64
	Err          error
}

// GeneratorStats is the rolling snapshot of a generate run used by the GUI to render
// progress, throughput, and the latest file.
type GeneratorStats struct {
	TotalFiles          int
	Processed           int
	WithErrors          int
	Skipped             int
	CurrentFileOrStatus string
	FileHashingProgress float64
	Speed               float64
}

// NewGeneratorStats returns stats initialized with a "ready" placeholder.
func NewGeneratorStats() GeneratorStats {
	return GeneratorStats{
		CurrentFileOrStatus: StatusReady,
	}
}

// Pending is derived as TotalFiles minus every terminal counter so the GUI can show
// "files remaining" without an extra field.
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

// VerifierStats is the rolling snapshot of a verify run used by the GUI to render
// progress, throughput, and the latest file.
type VerifierStats struct {
	TotalFiles          int
	Matched             int
	Mismatch            int
	Unreadable          int
	CurrentFileOrStatus string
	FileHashingProgress float64
	Speed               float64
}

// NewVerifierStats returns stats initialized with a "ready" placeholder.
func NewVerifierStats() VerifierStats {
	return VerifierStats{
		CurrentFileOrStatus: StatusReady,
	}
}

// Pending is derived as TotalFiles minus every terminal counter so the GUI can show
// "files remaining" without an extra field.
func (v VerifierStats) Pending() int { return v.TotalFiles - v.Matched - v.Mismatch - v.Unreadable }

// TotalProgress returns 0..1 fraction of files no longer pending.
func (v VerifierStats) TotalProgress() float64 {
	if v.TotalFiles == 0 {
		return 0
	}

	return float64(v.TotalFiles-v.Pending()) / float64(v.TotalFiles)
}

// DiscovererStats is the rolling snapshot of a discover run used by the GUI to render
// progress, throughput, and the latest file.
type DiscovererStats struct {
	TotalFiles          int
	Matched             int
	Mismatch            int
	Failed              int
	Skipped             int
	CurrentFileOrStatus string
	FileHashingProgress float64
	Speed               float64
}

// NewDiscovererStats returns stats initialized with a "ready" placeholder.
func NewDiscovererStats() DiscovererStats {
	return DiscovererStats{
		CurrentFileOrStatus: StatusReady,
	}
}

// Pending is derived as TotalFiles minus every terminal counter so the GUI can show
// "files remaining" without an extra field.
func (d DiscovererStats) Pending() int {
	return d.TotalFiles - d.Matched - d.Mismatch - d.Failed - d.Skipped
}

// TotalProgress returns 0..1 fraction of files no longer pending.
func (d DiscovererStats) TotalProgress() float64 {
	if d.TotalFiles == 0 {
		return 0
	}

	return float64(d.TotalFiles-d.Pending()) / float64(d.TotalFiles)
}

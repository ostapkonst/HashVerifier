package result

// VerifyStatusType classifies the per-file outcome of the verifier (sibling of GenerateStatusType and DiscoverStatusType).
type VerifyStatusType int

// Verifier outcome: matched, mismatched, or unreadable.
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

// Priority returns the sort key used by the GUI to order rows: a higher value is a more severe
// outcome, so descending order shows failures first.
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

// Generator outcome: hashed, skipped, or failed.
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

// Priority returns the sort key used by the GUI to order rows: a higher value is a more severe
// outcome, so descending order shows failures first.
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

// Color returns the Pango/GTK color name associated with this status (e.g. "firebrick1" for failure).
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

// DiscoverStatusType classifies the per-file outcome of the discoverer.
type DiscoverStatusType int

// Discoverer outcome: matched, mismatched, failed, a name without an embedded checksum, or a
// hidden file skipped by the scan.
const (
	DiscoverMatched DiscoverStatusType = iota
	DiscoverMismatch
	DiscoverFailed
	DiscoverNoChecksum
	DiscoverSkipped
)

// String returns the canonical uppercase label rendered in logs and tree views.
func (d DiscoverStatusType) String() string {
	switch d {
	case DiscoverMatched:
		return "MATCHED"
	case DiscoverMismatch:
		return "MISMATCH"
	case DiscoverFailed:
		return "FAILED"
	case DiscoverNoChecksum:
		return "NO CRC"
	case DiscoverSkipped:
		return "SKIPPED"
	default:
		panic("unknown status")
	}
}

// Priority returns the sort key used by the GUI to order rows: a higher value is a more severe
// outcome, so descending order shows failures first.
func (d DiscoverStatusType) Priority() int {
	switch d {
	case DiscoverMatched:
		return 0
	case DiscoverNoChecksum:
		return 1
	case DiscoverSkipped:
		return 2
	case DiscoverFailed:
		return 3
	case DiscoverMismatch:
		return 4
	default:
		return 5
	}
}

// Color returns the Pango/GTK color name associated with this status.
func (d DiscoverStatusType) Color() string {
	switch d {
	case DiscoverMatched:
		return "green"
	case DiscoverFailed:
		return "dark orange"
	case DiscoverMismatch:
		return "firebrick1"
	case DiscoverNoChecksum:
		return "steel blue"
	case DiscoverSkipped:
		return "gray"
	default:
		return ""
	}
}

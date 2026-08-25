package algorithm

// FormatType distinguishes CRC32/SFV (path-first) from *SUMS (hash-first) line layout in checksum files.
type FormatType int

// FormatType values for checksum-file line layout.
const (
	FormatHashFirst FormatType = iota
	FormatPathFirst
)

// FormatFromAlgorithm returns the line layout for algo (path-first for CRC32, hash-first otherwise).
func FormatFromAlgorithm(algo Algorithm) FormatType {
	switch algo {
	case CRC32:
		return FormatPathFirst
	default:
		return FormatHashFirst
	}
}

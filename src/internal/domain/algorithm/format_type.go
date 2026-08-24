package algorithm

type FormatType int

const (
	FormatHashFirst FormatType = iota
	FormatPathFirst
)

func FormatFromAlgorithm(algo Algorithm) FormatType {
	switch algo {
	case CRC32:
		return FormatPathFirst
	default:
		return FormatHashFirst
	}
}

package inventory

import "fmt"

const (
	kib = int64(1) << 10
	mib = int64(1) << 20
	gib = int64(1) << 30
	tib = int64(1) << 40
)

// FormatBytes renders a byte count in GiB or TiB with one decimal place.
// Negative values (the StorageUnknown sentinel) render as "unknown".
func FormatBytes(b int64) string {
	if b < 0 {
		return "unknown"
	}
	gibf := float64(b) / float64(gib)
	if b >= tib {
		return fmt.Sprintf("%.1fTiB", float64(b)/float64(tib))
	}
	return fmt.Sprintf("%.1fGiB", gibf)
}

// UsedBytes derives used capacity from total and available bytes, clamped to
// zero so inconsistent input cannot produce a negative used value.
func UsedBytes(total, available int64) int64 {
	used := total - available
	if used < 0 {
		return 0
	}
	return used
}

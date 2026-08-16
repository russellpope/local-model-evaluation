// Package format holds the pure presentation helpers shared by the CLI.
package format

import "fmt"

const (
	gib = int64(1) << 30
	tib = int64(1) << 40
)

// FormatBytes renders a byte count in GiB or TiB with one decimal place.
// Negative values clamp to zero so a bad upstream value cannot produce a
// negative size in the table.
func FormatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	if n >= tib {
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(tib))
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/float64(gib))
}

// UsedBytes computes used capacity as total - available, clamped into
// [0, total] so that inconsistent upstream values cannot produce a negative
// or over-total used figure.
func UsedBytes(total, available int64) int64 {
	used := total - available
	switch {
	case used < 0:
		return 0
	case used > total:
		return total
	default:
		return used
	}
}

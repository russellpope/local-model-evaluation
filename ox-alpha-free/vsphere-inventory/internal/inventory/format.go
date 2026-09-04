package inventory

import "fmt"

const (
	gib = 1 << 30
	tib = 1 << 40
)

// FormatBytes renders a byte count in GiB or TiB with one decimal place,
// e.g. "12.3 GiB" or "1.5 TiB".
func FormatBytes(b int64) string {
	if b >= tib {
		return fmt.Sprintf("%.1f TiB", float64(b)/tib)
	}
	return fmt.Sprintf("%.1f GiB", float64(b)/gib)
}

// UsedOf returns consumed capacity as capacity minus free space, clamped to
// zero so inconsistent backend values can never produce negative usage.
func UsedOf(capacity, free int64) int64 {
	if free > capacity || free < 0 {
		return 0
	}
	return capacity - free
}

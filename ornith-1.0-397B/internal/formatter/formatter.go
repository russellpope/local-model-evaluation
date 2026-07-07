package formatter

import (
	"fmt"
	"math"
)

// FormatBytes converts a byte count to a human-readable string in GiB or TiB
// with one decimal place.
func FormatBytes(bytes int64) string {
	const (
		gib = 1024 * 1024 * 1024
		tib = 1024 * 1024 * 1024 * 1024
	)
	if bytes >= tib {
		val := float64(bytes) / float64(tib)
		return fmt.Sprintf("%.1f TiB", math.Round(val*10)/10)
	}
	val := float64(bytes) / float64(gib)
	return fmt.Sprintf("%.1f GiB", math.Round(val*10)/10)
}

// UsedCapacity computes used = total - available, clamped to >= 0.
func UsedCapacity(total, available int64) int64 {
	used := total - available
	if used < 0 {
		return 0
	}
	return used
}

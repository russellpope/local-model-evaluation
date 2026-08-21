package inventory

import (
	"fmt"
)

// binary unit suffixes, smallest to largest.
var byteUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}

// formatBytes renders a byte count using binary (1024-based) units, choosing
// the largest unit that keeps the value >= 1 and one decimal place. Negative
// values are clamped to zero.
func formatBytes(b int64) string {
	if b < 0 {
		b = 0
	}

	const unit = 1024
	value := float64(b)
	idx := 0
	for value >= unit && idx < len(byteUnits)-1 {
		value /= unit
		idx++
	}

	if idx == 0 {
		return fmt.Sprintf("%d B", b)
	}
	return fmt.Sprintf("%.1f %s", value, byteUnits[idx])
}

// usedBytes returns used capacity given total and available (free) byte counts.
// It never returns a negative value, guarding against rounding or API quirks
// where reported free space exceeds reported capacity.
func usedBytes(total, available int64) int64 {
	if available > total {
		return 0
	}
	return total - available
}

// gbFromMB converts a megabyte value (VM configured memory) to gigabytes.
func gbFromMB(mb int64) float64 {
	return float64(mb) / 1024.0
}

// formatGB renders a gigabyte value with one decimal place.
func formatGB(gb float64) string {
	return fmt.Sprintf("%.1f GB", gb)
}

// FormatBytes renders a byte count for display using binary (1024-based) units
// with one decimal place, e.g. "9.5 GiB". It is the presentation wrapper over
// formatBytes used by the CLI layer.
func FormatBytes(b int64) string {
	return formatBytes(b)
}

// FormatRAMGB renders a virtual machine's configured memory in GB for display.
// Two decimal places keep small allocations (for example a 32 MB VM on vcsim)
// from rounding down to "0.0 GB". It is the presentation wrapper used by the
// CLI layer.
func FormatRAMGB(gb float64) string {
	return fmt.Sprintf("%.2f GB", gb)
}

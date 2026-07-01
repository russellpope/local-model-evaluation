package formatter

import (
	"fmt"
	"math"
)

// FormatBytes converts bytes to a human-readable string in GiB or TiB.
func FormatBytes(bytes int64) string {
	if bytes < 0 {
		return "0.0 B"
	}

	const (
		Byte = 1
		KiB  = 1024 * Byte
		MiB  = 1024 * KiB
		GiB  = 1024 * MiB
		TiB  = 1024 * GiB
	)

	switch {
	case bytes >= TiB:
		tib := float64(bytes) / float64(TiB)
		return fmt.Sprintf("%.1f TiB", tib)
	case bytes >= GiB:
		gib := float64(bytes) / float64(GiB)
		return fmt.Sprintf("%.1f GiB", gib)
	case bytes >= MiB:
		mib := float64(bytes) / float64(MiB)
		return fmt.Sprintf("%.1f MiB", mib)
	case bytes >= KiB:
		kib := float64(bytes) / float64(KiB)
		return fmt.Sprintf("%.1f KiB", kib)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatRAMGB formats RAM in GiB, showing fractional GiB for >= 0.01 GiB or MiB for smaller.
func FormatRAMGB(gb float64) string {
	if gb >= 1.0 {
		return fmt.Sprintf("%.1f", gb)
	}
	if gb >= 0.01 {
		return fmt.Sprintf("%.2f", gb)
	}
	mib := gb * 1024.0
	return fmt.Sprintf("%.0f MiB", mib)
}

// FormatBytesRounded is like FormatBytes but rounds to the nearest whole number for values < 1 GiB.
// Deprecated: Use FormatBytes for consistent units.
func FormatBytesRounded(bytes int64) string {
	if bytes < 0 {
		bytes = 0
	}

	const (
		Byte = 1
		KiB  = 1024 * Byte
		MiB  = 1024 * KiB
		GiB  = 1024 * MiB
		TiB  = 1024 * GiB
	)

	switch {
	case bytes >= TiB:
		tib := math.Round(float64(bytes)/float64(TiB)*10) / 10
		return fmt.Sprintf("%.1f TiB", tib)
	case bytes >= GiB:
		gib := math.Round(float64(bytes)/float64(GiB)*10) / 10
		return fmt.Sprintf("%.1f GiB", gib)
	case bytes >= MiB:
		mib := int64(math.Round(float64(bytes) / float64(MiB)))
		return fmt.Sprintf("%d MiB", mib)
	case bytes >= KiB:
		kib := int64(math.Round(float64(bytes) / float64(KiB)))
		return fmt.Sprintf("%d KiB", kib)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

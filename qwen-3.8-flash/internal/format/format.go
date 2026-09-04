// Package format provides human-readable unit formatting for table output.
package format

import (
	"fmt"
	"math"
)

const (
	KiB = 1 << 10
	MiB = 1 << 20
	GiB = 1 << 30
	TiB = 1 << 40
)

// HumanBytes renders a byte count as GiB or TiB with one decimal place,
// e.g. 107374182400 -> "100.0 GiB", 2.5 TiB -> "2.5 TiB".
func HumanBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	switch {
	case n >= TiB:
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(TiB))
	default:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	}
}

// MemoryGB renders a memory size given in MiB as GB with one decimal place.
func MemoryGB(mb int32) string {
	return fmt.Sprintf("%.1f GB", math.Max(float64(mb), 0)/1024)
}

// Used computes used capacity as total minus available, never negative.
func Used(total, available int64) int64 {
	used := total - available
	if used < 0 {
		return 0
	}
	return used
}

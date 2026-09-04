// Package format provides human-readable byte formatting helpers.
package format

import (
	"fmt"
)

const (
	gib = 1 << 30
	tib = 1 << 40
)

// Bytes renders a byte count as GiB or TiB with one decimal place.
// Values at or above 1 TiB use TiB; everything else uses GiB. Negative
// inputs are clamped to zero.
func Bytes(b int64) string {
	if b < 0 {
		b = 0
	}
	if b >= tib {
		return fmt.Sprintf("%.1f TiB", float64(b)/float64(tib))
	}
	return fmt.Sprintf("%.1f GiB", float64(b)/float64(gib))
}

// Used returns capacity minus free, clamped to [0, capacity].
func Used(capacity, free int64) int64 {
	if capacity < 0 {
		capacity = 0
	}
	if free < 0 {
		free = 0
	}
	if free > capacity {
		return 0
	}
	return capacity - free
}

// MemoryMB renders a memory size in MB as GB with up to one decimal place.
func MemoryMB(mb int64) string {
	return fmt.Sprintf("%g GB", float64(mb)/1024)
}

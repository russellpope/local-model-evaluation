// Package format renders inventory values as plain, greppable text.
package format

import "fmt"

const (
	gib = 1 << 30
	tib = 1 << 40
)

// Bytes renders a byte count as GiB/TiB with one decimal place. Values at or
// above 1 TiB render as TiB; everything else as GiB (small values therefore
// legitimately round to "0.0 GiB"). Negative inputs clamp to zero.
func Bytes(n int64) string {
	if n < 0 {
		n = 0
	}
	if n >= tib {
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(tib))
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/float64(gib))
}

// MemoryGB renders configured memory in MiB as GiB with one decimal place.
func MemoryGB(mb int64) string {
	if mb < 0 {
		mb = 0
	}
	return fmt.Sprintf("%.1f GiB", float64(mb)/1024.0)
}

// Used computes used capacity from total and free values, clamping
// nonsensical negative results (free > total) to zero.
func Used(total, free int64) int64 {
	if total <= 0 {
		return 0
	}
	if free < 0 {
		free = 0
	}
	if free > total {
		return 0
	}
	return total - free
}

// Package format renders machine values as consistent, greppable text.
package format

import "fmt"

// Bytes renders a byte count as GiB or TiB with one decimal place.
// Values at or above 1 TiB use TiB; everything else uses GiB.
func Bytes(b int64) string {
	const (
		GiB = 1 << 30
		TiB = 1 << 40
	)
	switch {
	case b < 0:
		return "0.0 GiB"
	case b >= TiB:
		return fmt.Sprintf("%.1f TiB", float64(b)/TiB)
	default:
		return fmt.Sprintf("%.1f GiB", float64(b)/GiB)
	}
}

// Used returns capacity - freeSpace, floored at zero.
func Used(capacity, free int64) int64 {
	used := capacity - free
	if used < 0 {
		return 0
	}
	return used
}

// GB renders a MiB count as GiB with one decimal place.
func GB(miB int64) string {
	return Bytes(miB << 20)
}

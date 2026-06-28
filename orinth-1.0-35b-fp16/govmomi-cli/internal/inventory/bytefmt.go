package inventory

import (
	"fmt"
)

const (
	giB = 1 << 30
	tiB = 1 << 40
)

// FormatBytes renders b bytes in human-readable GiB/TiB with one decimal place.
// Values < 1 GiB render as "X.X GiB". Negative or zero values still format; a
// truly negative input would indicate a programming error upstream and is
// reported verbatim rather than clamped — callers should guard against it.
func FormatBytes(b int64) string {
	if b >= tiB {
		return fmt.Sprintf("%.1f TiB", float64(b)/float64(tiB))
	}
	return fmt.Sprintf("%.1f GiB", float64(b)/float64(giB))
}

// UsedFromCapacity returns used bytes derived from a total capacity and free
// (available) space. It clamps the result to [0, capacity] so that floating-
// point rounding or inconsistent API reports cannot produce negative numbers.
func UsedFromCapacity(capacity, available int64) int64 {
	used := capacity - available
	if used < 0 {
		return 0
	}
	if used > capacity {
		return capacity
	}
	return used
}

// MBToBytes converts memory in MiB to bytes. Used for VM RAM reporting.
func MBToBytes(mib int64) int64 {
	return mib << 20
}

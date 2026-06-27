package inventory

import (
	"fmt"
	"math"
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
	if b >= giB {
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(giB))
	}
	return fmt.Sprintf("%.1f GiB", float64(b)/float64(giB))
}

// FormatGB renders b bytes as a GB value with one decimal place, using the
// binary definition (1 GB = 1 << 30 bytes) for consistency with FormatBytes.
func FormatGB(b int64) string {
	if b == 0 {
		return "0.0 GiB"
	}
	gb := float64(b) / float64(giB)
	s := fmt.Sprintf("%.1f", gb)
	// Strip trailing zeros after the decimal point but keep at least one digit.
	if i := indexOfDot(s); i >= 0 {
		end := len(s) - 1
		for end > i+1 && s[end] == '0' {
			end--
		}
		s = s[:end+1]
	}
	return s + " GiB"
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

// RoundHalfUp rounds v to n decimal places using "round half up" semantics,
// matching the standard formatting convention used by FormatGB callers.
func RoundHalfUp(v float64, n int) float64 {
	pow := math.Pow(10, float64(n))
	return math.Floor(v*pow+0.5) / pow
}

func indexOfDot(s string) int {
	for i, c := range s {
		if c == '.' {
			return i
		}
	}
	return -1
}

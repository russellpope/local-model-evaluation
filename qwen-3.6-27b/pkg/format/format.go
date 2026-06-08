package format

import (
	"fmt"
	"math"
)

const (
	_           = iota
	KiB float64 = 1 << (10 * iota)
	MiB
	GiB
	TiB
)

func HumanBytes(b int64) string {
	return HumanBytesFloat(float64(b))
}

func HumanBytesFloat(b float64) string {
	if b < 0 {
		return "0.0 GiB"
	}
	if b >= TiB {
		return fmt.Sprintf("%.1f TiB", b/TiB)
	}
	return fmt.Sprintf("%.1f GiB", b/GiB)
}

func GBString(ramMB uint64) string {
	ramGB := float64(ramMB) / 1024.0
	rounded := math.Round(ramGB*10) / 10
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%.0f", rounded)
	}
	return fmt.Sprintf("%.1f", rounded)
}

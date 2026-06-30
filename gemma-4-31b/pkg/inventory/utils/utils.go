package utils

import (
	"fmt"
)

func FormatBytes(bytes int64) string {
	const GiB = 1024 * 1024 * 1024
	const TiB = 1024 * GiB

	if bytes >= TiB {
		return fmt.Sprintf("%.1fTiB", float64(bytes)/float64(TiB))
	}
	return fmt.Sprintf("%.1fGiB", float64(bytes)/float64(GiB))
}

func FormatRAM(memMB int32) float64 {
	return float64(memMB) / 1024.0
}

package main

import (
	"fmt"
)

const (
	ByteGiB = 1024 * 1024 * 1024
	ByteTiB = 1024 * ByteGiB
)

func formatBytes(bytes int64) string {
	if bytes >= ByteTiB {
		return fmt.Sprintf("%.1f TiB", float64(bytes)/ByteTiB)
	}
	if bytes >= ByteGiB {
		return fmt.Sprintf("%.1f GiB", float64(bytes)/ByteGiB)
	}
	return fmt.Sprintf("%.1f MiB", float64(bytes)/(1024*1024))
}

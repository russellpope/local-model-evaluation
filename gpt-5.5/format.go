package main

import "fmt"

const (
	gib = 1 << 30
	tib = 1 << 40
)

func FormatBytes(n int64) string {
	if n >= tib {
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(tib))
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/float64(gib))
}

func UsedBytes(capacity, available int64) int64 {
	if available >= capacity {
		return 0
	}
	return capacity - available
}

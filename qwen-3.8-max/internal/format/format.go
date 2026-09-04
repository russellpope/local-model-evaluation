package format

import "fmt"

const (
	gib = int64(1 << 30)
	tib = int64(1 << 40)
)

func Bytes(n int64) string {
	if n < 0 {
		n = 0
	}
	if n >= tib {
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(tib))
	}
	return fmt.Sprintf("%.1f GiB", float64(n)/float64(gib))
}

func RAMMB(miB int64) string {
	return fmt.Sprintf("%.1f GB", float64(miB)/1024)
}

func Used(total, free int64) int64 {
	used := total - free
	if used < 0 {
		return 0
	}
	return used
}

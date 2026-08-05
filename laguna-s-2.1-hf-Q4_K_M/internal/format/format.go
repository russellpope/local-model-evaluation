package format

import "fmt"

const (
	MiB = 1024 * 1024
	GiB = 1024 * MiB
	TiB = GiB * 1024
)

func HumanBytes(bytes int64) string {
	return HumanBytesFloat(float64(bytes))
}

func HumanBytesFloat(bytes float64) string {
	switch {
	case bytes >= float64(TiB):
		return fmt.Sprintf("%.1f TiB", bytes/float64(TiB))
	case bytes >= float64(GiB):
		return fmt.Sprintf("%.1f GiB", bytes/float64(GiB))
	default:
		return fmt.Sprintf("%.1f MiB", bytes/float64(MiB))
	}
}

func GB(bytes int64) float64 {
	return float64(bytes) / float64(GiB)
}

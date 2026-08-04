package format

import "fmt"

func Bytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	if exp >= len(units) {
		return fmt.Sprintf("%.1f EiB", float64(b)/float64(div))
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

package format

import "fmt"

const unit = 1024

func HumanReadable(bytes int64) string {
	if bytes < unit {
		return fmt.Sprintf("%.1fB", float64(bytes))
	}
	div := int64(1)
	exp := 0
	for bytes/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	if exp >= len(units) {
		return "0B"
	}
	return fmt.Sprintf("%.1f%s", float64(bytes)/float64(div), units[exp])
}

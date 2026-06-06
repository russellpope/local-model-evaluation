package formatter

import (
	"fmt"
	"io"
	"strings"
)

func FormatBytes(b int64) string {
	const GiB = 1024 * 1024 * 1024
	const TiB = GiB * 1024

	if b < TiB {
		val := float64(b) / float64(GiB)
		return fmt.Sprintf("%.1f GiB", val)
	}
	val := float64(b) / float64(TiB)
	return fmt.Sprintf("%.1f TiB", val)
}

func PrintTable(w io.Writer, headers []string, rows [][]string) {
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
}

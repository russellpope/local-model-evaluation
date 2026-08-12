package output

import (
	"fmt"
	"io"
	"text/tabwriter"
)

func WriteTable(w io.Writer, header []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, formatRow(header))
	for _, r := range rows {
		_, _ = fmt.Fprintln(tw, formatRow(r))
	}
	_ = tw.Flush()
}

func formatRow(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += "\t"
		}
		out += c
	}
	return out
}

func BytesToHuman(bytes int64) string {
	if bytes <= 0 {
		return "0 GiB"
	}
	const GiB = 1024 * 1024 * 1024
	const TiB = GiB * 1024
	if bytes >= TiB {
		return fmt.Sprintf("%.1f TiB", float64(bytes)/float64(TiB))
	}
	return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(GiB))
}

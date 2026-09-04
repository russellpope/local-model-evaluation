package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// printTable writes a tab-separated table with a header row, rendered by
// text/tabwriter into aligned columns.
func printTable(w io.Writer, header []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(header, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

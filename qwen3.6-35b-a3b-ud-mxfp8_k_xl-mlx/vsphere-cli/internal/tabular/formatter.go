package tabular

import (
	"fmt"
	"io"
	"text/tabwriter"
)

type Formatter struct {
	w *tabwriter.Writer
}

func NewFormatter(out io.Writer) *Formatter {
	return &Formatter{
		w: tabwriter.NewWriter(out, 0, 0, 2, ' ', 0),
	}
}

func (f *Formatter) Flush() error {
	return f.w.Flush()
}

func (f *Formatter) PrintHeader(cols ...string) {
	for i, col := range cols {
		if i > 0 {
			fmt.Fprint(f.w, "\t")
		}
		fmt.Fprint(f.w, col)
	}
	fmt.Fprintln(f.w)
}

func (f *Formatter) PrintRow(vals ...string) {
	for i, val := range vals {
		if i > 0 {
			fmt.Fprint(f.w, "\t")
		}
		fmt.Fprint(f.w, val)
	}
	fmt.Fprintln(f.w)
}

func FormatBytes(bytes int64) string {
	const (
		GiB = 1024 * 1024 * 1024
		TiB = 1024 * GiB
	)

	if bytes >= TiB {
		return fmt.Sprintf("%.1f TiB", float64(bytes)/float64(TiB))
	}
	return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(GiB))
}

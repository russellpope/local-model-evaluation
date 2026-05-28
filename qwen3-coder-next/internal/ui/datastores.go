package ui

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/model"
)

func PrintDatastores(w io.Writer, dss []model.DatastoreInfo) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	defer tw.Flush()

	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")

	for _, ds := range dss {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ds.Name, ds.Type, ds.UsedHuman(), ds.AvailableHuman())
	}

	return nil
}

func init() {
	_ = fmt.Fprintln
}

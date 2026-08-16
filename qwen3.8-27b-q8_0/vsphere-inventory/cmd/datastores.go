package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores with transport type, used and available capacity",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, timeout, cleanup, err := connect(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		dss, err := inventory.ListDatastores(ctx, client.Client)
		if err != nil {
			return opError(timeout, err)
		}

		printDatastores(os.Stdout, dss)
		return nil
	},
}

func printDatastores(w io.Writer, dss []inventory.DatastoreInfo) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			ds.Name,
			ds.Type,
			format.FormatBytes(ds.Used),
			format.FormatBytes(ds.Available),
		)
	}
	_ = tw.Flush()
}

func init() {
	rootCmd.AddCommand(datastoresCmd)
}

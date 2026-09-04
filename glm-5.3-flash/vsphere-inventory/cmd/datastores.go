package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/internal/format"
	"vsphere-inventory/internal/inventory"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List datastores (transport, used/available capacity)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return connect(cmd, func(ctx context.Context, c *vim25.Client) error {
			rows, err := inventory.ListDatastores(ctx, c)
			if err != nil {
				return err
			}
			printDatastores(rows)
			return nil
		})
	},
}

func printDatastores(rows []inventory.DatastoreInfo) {
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "no datastores found")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			r.Name, r.Type,
			format.FormatBytes(r.UsedBytes), format.FormatBytes(r.AvailableBytes))
	}
}

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/inventory"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List datastores with backing transport, used and available capacity",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig(cmd)
		if err != nil {
			return err
		}
		return runWithClient(cfg, func(ctx context.Context, client *vim25.Client) error {
			dss, err := inventory.ListDatastores(ctx, client)
			if err != nil {
				return err
			}
			return printDatastores(os.Stdout, dss)
		})
	},
}

func printDatastores(w io.Writer, dss []inventory.DatastoreInfo) error {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			ds.Name,
			ds.Type,
			inventory.FormatBytes(ds.UsedBytes),
			inventory.FormatBytes(ds.AvailableBytes),
		)
	}
	return tw.Flush()
}

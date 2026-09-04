package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"vsphere-inventory/internal/format"
	"vsphere-inventory/internal/inventory"
)

func newDatastoresCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List all datastores with transport type and capacity",
		RunE: func(cmd *cobra.Command, args []string) error {
			infos, err := inventory.ListDatastores(a.ctx, a.client.Client)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
			for _, ds := range infos {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					ds.Name, ds.Transport, format.Bytes(ds.UsedBytes), format.Bytes(ds.AvailableBytes))
			}
			return w.Flush()
		},
	}
}

package command

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/example/vsphere-inventory/internal/inventory"
)

func newDatastoresCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "Report datastores and their storage transport",
		Long:  "Report every datastore with its underlying storage transport (FC, iSCSI, NVMe, or NFS) and its used and available capacity.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDatastores(cmd)
		},
	}
}

func runDatastores(cmd *cobra.Command) error {
	ctx := cmd.Context()
	client, cleanup, err := connect(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	dss, err := inventory.ListDatastores(ctx, client)
	if err != nil {
		return err
	}

	out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(out, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n",
			ds.Name, ds.Type, inventory.FormatBytes(ds.UsedBytes), inventory.FormatBytes(ds.AvailableBytes))
	}
	return out.Flush()
}

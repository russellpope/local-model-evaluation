package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	vim25 "github.com/vmware/govmomi/vim25"
	"govmomi-cli/internal/inventory"

	"github.com/spf13/cobra"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores with transport type and capacity",
	Long:  `Enumerate every datastore in the inventory and print NAME, TYPE (transport protocol), USED and AVAILABLE.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithClient(cmd, func(ctx context.Context, cli *vim25.Client) error {
			dsList, err := inventory.ListDatastores(ctx, cli)
			if err != nil {
				return fmt.Errorf("listing datastores: %w", err)
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE"); err != nil {
				return err
			}
			for _, ds := range dsList {
				used := inventory.UsedFromCapacity(ds.CapacityB, ds.FreeB)
				if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
					ds.Name, ds.Type,
					inventory.FormatBytes(used),
					inventory.FormatBytes(ds.FreeB)); err != nil {
					return err
				}
			}
			return tw.Flush()
		})
	},
}

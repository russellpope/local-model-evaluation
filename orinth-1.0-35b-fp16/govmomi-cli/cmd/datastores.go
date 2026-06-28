package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"govmomi-cli/internal/inventory"

	"github.com/spf13/cobra"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores with transport type and capacity",
	Long:  `Enumerate every datastore in the inventory and print NAME, TYPE (transport protocol), USED and AVAILABLE.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getConfig()
		if err != nil {
			return err
		}

		rootCtx := cmd.Context()
		ctx, cancel := context.WithTimeout(rootCtx, cfg.Timeout)
		defer cancel()

		cli, sm, err := newClient(ctx, cfg)
		if err != nil {
			return err
		}
		defer closeClient(ctx, cli, sm)

		dsList, err := inventory.ListDatastores(ctx, cli)
		if err != nil {
			return fmt.Errorf("listing datastores: %w", err)
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
		for _, ds := range dsList {
			used := inventory.UsedFromCapacity(ds.CapacityB, ds.FreeB)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				ds.Name, ds.Type,
				inventory.FormatBytes(used),
				inventory.FormatBytes(ds.FreeB))
		}
		return tw.Flush()
	},
}

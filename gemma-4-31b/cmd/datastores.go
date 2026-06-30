package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"govmomi-cli/pkg/inventory"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := GetConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
		defer cancel()

		client, err := inventory.NewClient(ctx, cfg)
		if err != nil {
			return err
		}

		dstores, err := inventory.GetDatastores(ctx, client.Client)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
		for _, ds := range dstores {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ds.Name, ds.Type, ds.Used, ds.Available)
		}
		return w.Flush()
	},
}

func init() {
	RootCmd.AddCommand(datastoresCmd)
}

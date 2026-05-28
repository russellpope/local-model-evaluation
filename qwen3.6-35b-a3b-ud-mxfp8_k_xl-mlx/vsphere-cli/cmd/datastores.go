package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-cli/internal/tabular"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, logout, err := withClient(cmd.Context(), cmd, args)
		if err != nil {
			return err
		}
		defer logout()

		datastores, err := client.GetDatastores(ctx)
		if err != nil {
			return fmt.Errorf("getting datastores: %w", err)
		}

		f := tabular.NewFormatter(os.Stdout)
		defer f.Flush()

		f.PrintHeader("NAME", "TYPE", "USED", "AVAILABLE")

		for _, ds := range datastores {
			f.PrintRow(
				ds.Name,
				ds.Transport,
				tabular.FormatBytes(ds.Used),
				tabular.FormatBytes(ds.Available),
			)
		}

		return nil
	},
}

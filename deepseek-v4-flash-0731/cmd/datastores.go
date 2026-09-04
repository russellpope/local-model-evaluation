package cmd

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"vint/format"
	"vint/inventory"
)

func newDatastoresCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List all datastores (name, transport type, used, available)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := resolved(cmd)
			if err != nil {
				return err
			}
			if err := requireURL(cfg); err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
			defer cancel()

			c, err := inventory.Connect(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
			if err != nil {
				return err
			}
			defer c.Logout(ctx)

			dss, err := inventory.ListDatastores(ctx, c.Client)
			if err != nil {
				return fmt.Errorf("datastores: %w", err)
			}
			return renderDatastores(cmd.OutOrStdout(), dss)
		},
	}
}

// renderDatastores writes the datastore table:
//
//	NAME  TYPE  USED  AVAILABLE
func renderDatastores(w io.Writer, dss []inventory.DatastoreInfo) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			ds.Name, ds.Type, format.Bytes(ds.Used), format.Bytes(ds.Free))
	}
	return tw.Flush()
}

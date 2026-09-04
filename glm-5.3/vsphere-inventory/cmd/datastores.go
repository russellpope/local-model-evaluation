package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"vsphere-inventory/internal/inventory"
	"vsphere-inventory/internal/render"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores",
	Long: "List all datastores with the underlying storage transport " +
		"(FC, iSCSI, NVMe or NFS), used and available capacity.",
	RunE: func(cmd *cobra.Command, args []string) error {
		dss, err := inventory.ListDatastores(cmd.Context(), app.client)
		if err != nil {
			return err
		}
		if err := render.WriteDatastores(cmd.OutOrStdout(), dss); err != nil {
			return fmt.Errorf("write datastores table: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(datastoresCmd)
}

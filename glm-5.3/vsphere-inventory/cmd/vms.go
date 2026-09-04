package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"vsphere-inventory/internal/inventory"
	"vsphere-inventory/internal/render"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines",
	Long: "List all virtual machines in the inventory with configured vCPU, " +
		"memory (GB) and consumed (committed) storage.",
	RunE: func(cmd *cobra.Command, args []string) error {
		vms, err := inventory.ListVMs(cmd.Context(), app.client)
		if err != nil {
			return err
		}
		if err := render.WriteVMs(cmd.OutOrStdout(), vms); err != nil {
			return fmt.Errorf("write vms table: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(vmsCmd)
}

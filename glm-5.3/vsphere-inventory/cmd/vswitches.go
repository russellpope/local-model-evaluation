package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"vsphere-inventory/internal/inventory"
	"vsphere-inventory/internal/render"
)

var vswitchesPortgroup string

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List virtual switches and port groups",
	Long: "List all virtual switches — standard (host vSwitches) and " +
		"distributed (vDS) — along with their port groups, VLANs, uplinks, " +
		"LACP state and port usage.\n\n" +
		"With --portgroup, list the virtual machines connected to the named " +
		"port group instead (works for standard and distributed port groups).",
	RunE: func(cmd *cobra.Command, args []string) error {
		if vswitchesPortgroup != "" {
			vms, err := inventory.ListPortGroupVMs(cmd.Context(), app.client, vswitchesPortgroup)
			if err != nil {
				return err
			}
			if err := render.WriteVMs(cmd.OutOrStdout(), vms); err != nil {
				return fmt.Errorf("write port group vms table: %w", err)
			}
			return nil
		}

		switches, err := inventory.ListSwitches(cmd.Context(), app.client)
		if err != nil {
			return err
		}
		if err := render.WriteSwitches(cmd.OutOrStdout(), switches); err != nil {
			return fmt.Errorf("write switches table: %w", err)
		}
		return nil
	},
}

func init() {
	vswitchesCmd.Flags().StringVar(&vswitchesPortgroup, "portgroup", "",
		"list the virtual machines connected to this port group instead of the switch listing")
	rootCmd.AddCommand(vswitchesCmd)
}

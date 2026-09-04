package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ldh/vsphere-inventory/internal/inventory"
)

func newVSwitchesCmd(v *viper.Viper) *cobra.Command {
	var portgroup string
	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List virtual switches and port groups, or VMs on a port group",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connect(cmd, v)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			if portgroup != "" {
				vms, err := inventory.VMsOnPortgroup(conn.Ctx, conn.Vim(), portgroup)
				if err != nil {
					return err
				}
				return printTable(stdout, "NAME\tVCPU\tRAM\tSTORAGE", vmRows(vms))
			}

			pgs, err := inventory.Switches(conn.Ctx, conn.Vim())
			if err != nil {
				return err
			}
			return printTable(stdout,
				"SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED",
				portgroupRows(pgs))
		},
	}
	cmd.Flags().StringVar(&portgroup, "portgroup", "", "list VMs connected to this port group instead of the switch table")
	return cmd
}

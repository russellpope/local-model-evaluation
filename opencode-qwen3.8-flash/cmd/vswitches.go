package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

func newVSwitchesCommand(cfg *config.Config) *cobra.Command {
	var portgroup string

	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List standard and distributed virtual switches with their port groups",
		Long: `List every virtual switch (standard and distributed) with its port
groups, VLAN, uplinks, LACP state, and port usage.

With --portgroup <name>, list the virtual machines connected to that port
group instead.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connect(cmd, cfg)
			if err != nil {
				return err
			}
			defer conn.close()

			if portgroup != "" {
				vms, err := inventory.FetchVMsByPortgroup(conn.ctx, conn.vim25(), portgroup)
				if err != nil {
					return fmt.Errorf("look up portgroup %q: %w", portgroup, err)
				}
				return renderVMs(cmd.OutOrStdout(), vms)
			}

			switches, err := inventory.FetchSwitches(conn.ctx, conn.vim25())
			if err != nil {
				return fmt.Errorf("list virtual switches: %w", err)
			}
			return renderSwitches(cmd.OutOrStdout(), switches)
		},
	}
	cmd.Flags().StringVar(&portgroup, "portgroup", "", "list the VMs connected to this port group instead of the switch listing")
	return cmd
}

package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"vsphere-inventory/internal/inventory"
)

func newVSwitchesCmd(a *app) *cobra.Command {
	var portgroup string

	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List standard and distributed virtual switches with their port groups",
		Long: "List standard and distributed virtual switches with their port groups.\n" +
			"With --portgroup <name>, list the virtual machines connected to that port group instead.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if portgroup != "" {
				return runPortgroupLookup(cmd, a, portgroup)
			}
			rows, err := inventory.ListSwitches(a.ctx, a.client.Client)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
			fmt.Fprintln(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
			for _, row := range rows {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
					row.Switch, row.SwitchType, row.Portgroup, row.VLAN, row.Uplinks, row.LACP, row.Ports, row.Used)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&portgroup, "portgroup", "", "list the VMs connected to this port group instead of the switch listing")
	return cmd
}

func runPortgroupLookup(cmd *cobra.Command, a *app, portgroup string) error {
	infos, err := inventory.VMsOnPortgroup(a.ctx, a.client.Client, portgroup)
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
	fmt.Fprintln(w, "NAME")
	for _, vm := range infos {
		fmt.Fprintln(w, vm.Name)
	}
	return w.Flush()
}

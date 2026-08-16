package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

var vswitchPortgroupName string

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List all virtual switches with their port groups, or the VMs on a port group",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, timeout, cleanup, err := connect(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		if vswitchPortgroupName != "" {
			vms, err := inventory.VMsInPortgroup(ctx, client.Client, vswitchPortgroupName)
			if err != nil {
				return opError(timeout, err)
			}
			printPortgroupVMs(os.Stdout, vms)
			return nil
		}

		switches, err := inventory.ListSwitches(ctx, client.Client)
		if err != nil {
			return opError(timeout, err)
		}

		printSwitches(os.Stdout, switches)
		return nil
	},
}

func printSwitches(w io.Writer, switches []inventory.SwitchInfo) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, s := range switches {
		for _, pg := range s.PortGroups {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				s.Name,
				s.Kind,
				pg.Name,
				pg.Vlan,
				pg.Uplinks,
				pg.LACP,
				pg.TotalPorts,
				pg.UsedPorts,
			)
		}
	}
	_ = tw.Flush()
}

func printPortgroupVMs(w io.Writer, vms []inventory.VMInfo) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME")
	for _, vm := range vms {
		fmt.Fprintln(tw, vm.Name)
	}
	_ = tw.Flush()
}

func init() {
	vswitchesCmd.Flags().StringVar(&vswitchPortgroupName, "portgroup", "",
		"list the virtual machines connected to this port group instead of the switch listing")
	rootCmd.AddCommand(vswitchesCmd)
}

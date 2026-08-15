package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/inventory"
)

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List virtual switches and port groups, or the VMs in one port group",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig(cmd)
		if err != nil {
			return err
		}
		portgroup, _ := cmd.Flags().GetString("portgroup")
		return runWithClient(cfg, func(ctx context.Context, client *vim25.Client) error {
			if portgroup != "" {
				vms, err := inventory.VMsInPortgroup(ctx, client, portgroup)
				if err != nil {
					return err
				}
				return printPortgroupVMs(os.Stdout, portgroup, vms)
			}
			switches, err := inventory.ListSwitches(ctx, client)
			if err != nil {
				return err
			}
			return printSwitches(os.Stdout, switches)
		})
	},
}

func init() {
	vswitchesCmd.Flags().String("portgroup", "", "list the virtual machines connected to this port group instead of listing switches")
}

func printSwitches(w io.Writer, switches []inventory.SwitchInfo) error {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, s := range switches {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			s.Switch,
			s.SwitchType,
			s.Portgroup,
			s.VLAN,
			s.Uplinks,
			s.LACP,
			s.Ports,
			s.Used,
		)
	}
	return tw.Flush()
}

func printPortgroupVMs(w io.Writer, portgroup string, vms []inventory.VMInfo) error {
	fmt.Fprintf(w, "VMs in port group %q:\n", portgroup)
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			vm.Name,
			vm.VCPU,
			inventory.FormatBytes(vm.RAMBytes),
			inventory.FormatBytes(vm.StorageBytes),
		)
	}
	return tw.Flush()
}

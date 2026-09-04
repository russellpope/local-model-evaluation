package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/internal/inventory"
)

var portgroupName string

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List standard and distributed switches and their port groups",
	Long: `List standard (host vSwitches) and distributed (vDS) switches with
their port groups: VLAN, uplinks, LACP state and port usage.

With --portgroup <name>, print the VMs attached to that port group instead
(works for both standard and distributed port groups).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if portgroupName != "" {
			return connect(cmd, func(ctx context.Context, c *vim25.Client) error {
				rows, err := inventory.VMsOnPortgroup(ctx, c, portgroupName)
				if err != nil {
					return err
				}
				printPortgroupVMs(portgroupName, rows)
				return nil
			})
		}
		return connect(cmd, func(ctx context.Context, c *vim25.Client) error {
			rows, err := inventory.ListSwitches(ctx, c)
			if err != nil {
				return err
			}
			printSwitches(rows)
			return nil
		})
	},
}

func init() {
	vswitchesCmd.Flags().StringVar(&portgroupName, "portgroup", "",
		"list the VMs connected to this port group (standard or distributed)")
}

func printSwitches(rows []inventory.SwitchInfo) {
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "no virtual switches found")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, s := range rows {
		pgs := s.PortGroups
		if len(pgs) == 0 {
			pgs = []inventory.PortGroupInfo{{Name: "-"}}
		}
		for _, pg := range pgs {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				s.Name, s.Type, pg.Name, pg.VLAN, s.Uplinks, s.LACP, s.Ports, s.Used)
		}
	}
}

func printPortgroupVMs(pg string, rows []inventory.VMInfo) {
	if len(rows) == 0 {
		fmt.Fprintf(os.Stderr, "no virtual machines connected to port group %q\n", pg)
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "NAME")
	for _, r := range rows {
		fmt.Fprintln(w, r.Name)
	}
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	vim25 "github.com/vmware/govmomi/vim25"
	"govmomi-cli/internal/inventory"

	"github.com/spf13/cobra"
)

var pgName string // --portgroup flag value

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List virtual switches and port groups, or VMs connected to a port group",
	Long: `Enumerate every standard and distributed virtual switch in the inventory
and print SWITCH, SWITCH TYPE, PORTGROUP, VLAN, UPLINKS, LACP, TOTAL PORTS, USED.

With --portgroup <name>, list VMs connected to that named port group instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getConfig()
		if err != nil {
			return err
		}

		rootCtx := cmd.Context()
		ctx, cancel := context.WithTimeout(rootCtx, cfg.Timeout)
		defer cancel()

		cli, sm, err := newClient(ctx, cfg)
		if err != nil {
			return err
		}
		defer closeClient(ctx, cli, sm)

		if pgName != "" {
			return runPortgroupMode(ctx, cli, pgName)
		}

		return runSwitchesMode(ctx, cli)
	},
}

func init() {
	vswitchesCmd.Flags().StringVar(&pgName, "portgroup", "", "list VMs connected to this port group (standard or distributed)")
}

// runSwitchesMode prints the standard + distributed switch listing.
func runSwitchesMode(ctx context.Context, cli *vim25.Client) error {
	switches, err := inventory.ListSwitches(ctx, cli)
	if err != nil {
		return fmt.Errorf("listing vswitches: %w", err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SWITCH\tHOST\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tTOTAL PORTS\tUSED")
	for _, s := range switches {
		host := s.Host
		if host == "" {
			host = "N/A"
		}
		used := s.UsedPorts
		if !s.UsedPortsValid {
			used = -1 // sentinel; rendered below
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			s.Switch, host, s.SwitchType, s.PortGroup, s.VLAN, s.Uplinks, s.LACP, s.TotalPorts, formatUsedPorts(used, s.UsedPortsValid))
	}
	return tw.Flush()
}

// runPortgroupMode prints VMs connected to the named port group.
func runPortgroupMode(ctx context.Context, cli *vim25.Client, pg string) error {
	vms, err := inventory.ListVMsByPortGroup(ctx, cli, pg)
	if err != nil {
		return fmt.Errorf("listing VMs for port group %q: %w", pg, err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM (GiB)\tSTORAGE")
	for _, v := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%.1f GiB\t%s\n",
			v.Name, v.VCPUs, float64(v.MemoryMB)/1024.0, inventory.FormatBytes(v.StorageB))
	}
	return tw.Flush()
}

// formatUsedPorts renders the USED column value. When the underlying data is
// not derivable (e.g. DVS port groups where AvailablePorts is not exposed by
// the API), renders "N/A" rather than a misleading numeric value.
func formatUsedPorts(used int32, valid bool) string {
	if !valid {
		return "N/A"
	}
	return strconv.Itoa(int(used))
}

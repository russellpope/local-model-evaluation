package command

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/example/vsphere-inventory/internal/inventory"
	"github.com/vmware/govmomi/vim25"
)

func newVswitchesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "Report standard and distributed virtual switches and their port groups",
		Long: "Report every virtual switch -- both standard (host vSwitches) and distributed (vDS) -- with their port groups. " +
			"Pass --portgroup to instead list the virtual machines connected to a single port group.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runVswitches(cmd)
		},
	}
	cmd.Flags().String("portgroup", "", "list the virtual machines connected to the named port group instead of the switch table")
	return cmd
}

func runVswitches(cmd *cobra.Command) error {
	ctx := cmd.Context()
	portgroup, err := cmd.Flags().GetString("portgroup")
	if err != nil {
		return err
	}

	client, cleanup, err := connect(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	if portgroup != "" {
		return runPortGroupLookup(ctx, client, portgroup, cmd.OutOrStdout())
	}
	return runSwitchTable(ctx, client, cmd.OutOrStdout())
}

func runSwitchTable(ctx context.Context, client *vim25.Client, w io.Writer) error {
	rows, err := inventory.ListSwitches(ctx, client)
	if err != nil {
		return err
	}

	out := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(out, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, r := range rows {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			r.Switch, r.SwitchType, r.PortGroup, r.VLAN, r.Uplinks, r.LACP, r.Ports, r.UsedPorts)
	}
	return out.Flush()
}

func runPortGroupLookup(ctx context.Context, client *vim25.Client, name string, w io.Writer) error {
	vms, err := inventory.VMsForPortGroup(ctx, client, name)
	if err != nil {
		return err
	}

	out := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(out, "NAME")
	for _, vm := range vms {
		fmt.Fprintf(out, "%s\n", vm.Name)
	}
	return out.Flush()
}

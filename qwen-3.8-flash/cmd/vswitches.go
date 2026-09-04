package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"local-model-evaluation/govmomi-cli/internal/client"
	"local-model-evaluation/govmomi-cli/internal/config"
	"local-model-evaluation/govmomi-cli/internal/inventory"
)

func newVSwitchesCommand(cfg *config.Config) *cobra.Command {
	var portgroup string

	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List standard and distributed virtual switches with their port groups, or the VMs on one port group",
		Args:  cobra.NoArgs,
		RunE: runWithClient(cfg, func(ctx context.Context, out io.Writer, cl *client.Client) error {
			if portgroup != "" {
				vms, err := inventory.VMsOnPortgroup(ctx, cl.Vim(), portgroup)
				if err != nil {
					return err
				}
				renderVMs(out, vms)
				return nil
			}
			rows, err := inventory.ListSwitches(ctx, cl.Vim())
			if err != nil {
				return err
			}
			renderSwitches(out, rows)
			return nil
		}),
	}
	cmd.Flags().StringVar(&portgroup, "portgroup", "", "list virtual machines connected to this port group instead of the switch listing")
	return cmd
}

func renderSwitches(out io.Writer, rows []inventory.SwitchInfo) {
	w := newTable(out)
	fmt.Fprintln(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			r.Switch,
			r.SwitchType,
			r.Portgroup,
			r.VLAN,
			r.Uplinks,
			r.LACP,
			strconv.FormatInt(r.Ports, 10),
			strconv.FormatInt(r.Used, 10))
	}
	_ = w.Flush()
}

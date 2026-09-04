package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"

	"vsphere-inventory/internal/inventory"
)

// connect resolves configuration, then returns a context bounded by the
// configured timeout and an authenticated client.
func connect(cmd *cobra.Command) (context.Context, context.CancelFunc, *govmomi.Client, error) {
	cfg, err := loadConfig(cmd)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	c, err := inventory.Connect(ctx, cfg)
	if err != nil {
		cancel()
		return nil, nil, nil, err
	}
	return ctx, cancel, c, nil
}

// finish logs out the client (with a detached context so cleanup survives
// cancellation) and releases the context timer.
func finish(c *govmomi.Client, cancel context.CancelFunc) {
	if c != nil {
		_ = c.Logout(context.Background())
	}
	cancel()
}

func newVmsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines with vCPU, RAM and consumed storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel, c, err := connect(cmd)
			if err != nil {
				return err
			}
			defer finish(c, cancel)

			vms, err := inventory.ListVMs(ctx, c.Client)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(vms))
			for _, vm := range vms {
				rows = append(rows, []string{
					vm.Name,
					strconv.FormatInt(int64(vm.VCPU), 10),
					fmt.Sprintf("%.1f", vm.RAMGB),
					inventory.FormatBytes(vm.StorageBytes),
				})
			}
			return printTable(os.Stdout, []string{"NAME", "VCPU", "RAM", "STORAGE"}, rows)
		},
	}
}

func newDatastoresCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List datastores with transport type, used and available capacity",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel, c, err := connect(cmd)
			if err != nil {
				return err
			}
			defer finish(c, cancel)

			dss, err := inventory.ListDatastores(ctx, c.Client)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(dss))
			for _, ds := range dss {
				rows = append(rows, []string{
					ds.Name,
					ds.Type,
					inventory.FormatBytes(ds.UsedBytes),
					inventory.FormatBytes(ds.AvailableBytes),
				})
			}
			return printTable(os.Stdout, []string{"NAME", "TYPE", "USED", "AVAILABLE"}, rows)
		},
	}
}

func newVswitchesCmd() *cobra.Command {
	var portgroup string
	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List standard and distributed switches with their port groups",
		Long: `List all standard (host vSwitches) and distributed (vDS) switches with
their port groups. With --portgroup NAME, instead list the virtual machines
connected to that port group.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel, c, err := connect(cmd)
			if err != nil {
				return err
			}
			defer finish(c, cancel)

			if portgroup != "" {
				vms, err := inventory.ListPortGroupVMs(ctx, c.Client, portgroup)
				if err != nil {
					return err
				}
				rows := make([][]string, 0, len(vms))
				for _, vm := range vms {
					rows = append(rows, []string{vm.Name})
				}
				return printTable(os.Stdout, []string{"NAME"}, rows)
			}

			switches, err := inventory.ListSwitches(ctx, c.Client)
			if err != nil {
				return err
			}
			rows := make([][]string, 0, len(switches))
			for _, sw := range switches {
				rows = append(rows, []string{
					sw.Switch,
					sw.SwitchType,
					sw.PortGroup,
					sw.VLAN,
					sw.Uplinks,
					sw.LACP,
					strconv.FormatInt(int64(sw.TotalPorts), 10),
					strconv.FormatInt(int64(sw.UsedPorts), 10),
				})
			}
			return printTable(os.Stdout,
				[]string{"SWITCH", "SWITCH TYPE", "PORTGROUP", "VLAN", "UPLINKS", "LACP", "PORTS", "USED"}, rows)
		},
	}
	cmd.Flags().StringVar(&portgroup, "portgroup", "", "list VMs connected to this port group")
	return cmd
}

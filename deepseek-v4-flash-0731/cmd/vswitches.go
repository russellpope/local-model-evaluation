package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"vint/config"
	"vint/format"
	"vint/inventory"
)

const flagPortGroup = "portgroup"

func newVSwitchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List all virtual switches and their port groups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := resolved(cmd)
			if err != nil {
				return err
			}
			if err := requireURL(cfg); err != nil {
				return err
			}
			if name, _ := cmd.Flags().GetString(flagPortGroup); name != "" {
				return runPortGroupLookup(cmd, cfg, name)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
			defer cancel()

			c, err := inventory.Connect(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
			if err != nil {
				return err
			}
			defer c.Logout(ctx)

			switches, err := inventory.ListSwitches(ctx, c.Client)
			if err != nil {
				return fmt.Errorf("vswitches: %w", err)
			}
			return renderSwitches(cmd.OutOrStdout(), switches)
		},
	}
	cmd.Flags().String(flagPortGroup, "", "list the VMs connected to the named port group instead of the switch table")
	return cmd
}

// runPortGroupLookup implements `vswitches --portgroup <name>` for both
// standard and distributed port groups.
func runPortGroupLookup(cmd *cobra.Command, cfg *config.Config, name string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	defer cancel()

	c, err := inventory.Connect(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
	if err != nil {
		return err
	}
	defer c.Logout(ctx)

	vms, err := inventory.VMsOnPortGroup(ctx, c.Client, name)
	if err != nil {
		return fmt.Errorf("vswitches --portgroup %q: %w", name, err)
	}
	return renderPortGroupVMs(cmd.OutOrStdout(), name, vms)
}

func renderSwitches(w io.Writer, switches []inventory.SwitchInfo) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, s := range switches {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			s.Switch, s.SwitchType, s.PortGroup, s.Vlan,
			strings.Join(s.Uplinks, ","), s.LACP, s.Ports, s.Used)
	}
	return tw.Flush()
}

func renderPortGroupVMs(w io.Writer, name string, vms []inventory.VMInfo) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "# virtual machines on port group %q\n", name)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM (GB)")
	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\n",
			vm.Name, vm.CPUs, format.GigabytesFromMiB(vm.RAMMB))
	}
	return tw.Flush()
}

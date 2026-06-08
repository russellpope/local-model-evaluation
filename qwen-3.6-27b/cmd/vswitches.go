package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"

	"github.com/example/vsphere-inventory-cli/pkg/config"
	"github.com/example/vsphere-inventory-cli/pkg/format"
	"github.com/example/vsphere-inventory-cli/pkg/inventory"
)

func newSwitchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List all virtual switches",
		Long:  "List all virtual switches (standard and distributed) with their port groups and configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSwitches(cmd.Context(), cmd)
		},
	}

	cmd.Flags().StringP("portgroup", "p", "", "List VMs connected to a specific port group")

	return cmd
}

func runSwitches(ctx context.Context, cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if err := config.Validate(cfg); err != nil {
		return err
	}

	portgroup := cmd.Flags().Lookup("portgroup").Value.String()

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client, cleanup, err := connectClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	if portgroup != "" {
		return runPortgroupVMs(ctx, client, portgroup)
	}

	return runSwitchesList(ctx, client)
}

func runSwitchesList(ctx context.Context, client *govmomi.Client) error {
	switches, err := inventory.ListSwitches(ctx, client)
	if err != nil {
		return fmt.Errorf("list switches: %w", err)
	}

	sort.Slice(switches, func(i, j int) bool {
		return switches[i].Name < switches[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SWITCH\tTYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")

	for _, sw := range switches {
		if len(sw.PortGroups) == 0 {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				sw.Name,
				sw.SwitchType,
				"-",
				"-",
				sw.Uplinks,
				sw.LACP,
				sw.TotalPorts,
				sw.UsedPorts,
			)
			continue
		}

		for i, pg := range sw.PortGroups {
			switchName := sw.Name
			if i > 0 {
				switchName = ""
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				switchName,
				sw.SwitchType,
				pg.Name,
				pg.VLAN,
				sw.Uplinks,
				sw.LACP,
				sw.TotalPorts,
				sw.UsedPorts,
			)
		}
	}

	w.Flush()
	return nil
}

func runPortgroupVMs(ctx context.Context, client *govmomi.Client, portgroupName string) error {
	vms, err := inventory.FindVMsInPortGroup(ctx, client, portgroupName)
	if err != nil {
		return fmt.Errorf("find VMs in port group %q: %w", portgroupName, err)
	}

	if len(vms) == 0 {
		fmt.Fprintf(os.Stdout, "No VMs found in port group %q\n", portgroupName)
		return nil
	}

	sort.Slice(vms, func(i, j int) bool {
		return vms[i].Name < vms[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVCPU\tRAM (GB)\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			vm.Name,
			vm.VCPU,
			format.GBString(uint64(vm.RAMMB)),
			format.HumanBytes(vm.Storage),
		)
	}
	w.Flush()

	return nil
}

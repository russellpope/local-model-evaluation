package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/client"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/config"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/inventory"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/output"
)

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List vSwitches or VMs for port group",
	RunE: func(cmd *cobra.Command, args []string) error {
		portGroup, _ := cmd.Flags().GetString("portgroup")
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		c, err := client.New(ctx, cfg)
		if err != nil {
			return err
		}
		defer client.Logout(ctx, c)
		if portGroup != "" {
			vms, err := inventory.GetVMsForPortGroup(ctx, c, portGroup)
			if err != nil {
				return err
			}
			header := []string{"NAME", "VCPU", "RAM", "STORAGE"}
			rows := make([][]string, 0, len(vms))
			for _, vm := range vms {
				rows = append(rows, []string{vm.Name, fmt.Sprintf("%d", vm.VCPU), fmt.Sprintf("%.1f GiB", vm.RAMGiB), vm.Storage})
			}
			output.WriteTable(os.Stdout, header, rows)
			return nil
		}
		switches, err := inventory.GetSwitches(ctx, c)
		if err != nil {
			return err
		}
		header := []string{"SWITCH", "SWITCH TYPE", "PORTGROUP", "VLAN", "UPLINKS", "LACP", "PORTS", "USED"}
		rows := make([][]string, 0, len(switches))
		for _, s := range switches {
			rows = append(rows, []string{s.SwitchName, s.SwitchType, s.PortGroup, s.VLAN, s.Uplinks, s.LACP, fmt.Sprintf("%d", s.Ports), fmt.Sprintf("%d", s.Used)})
		}
		output.WriteTable(os.Stdout, header, rows)
		return nil
	},
}

func init() {
	vswitchesCmd.Flags().String("portgroup", "", "Port group name")
}

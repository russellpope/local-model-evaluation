package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-cli/internal/tabular"
)

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List all virtual switches and port groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, logout, err := withClient(cmd.Context(), cmd, args)
		if err != nil {
			return err
		}
		defer logout()

		portgroup := cmd.Flag("portgroup").Value.String()
		if portgroup != "" {
			vms, err := client.GetVMsByPortgroup(ctx, portgroup)
			if err != nil {
				return fmt.Errorf("getting VMs by portgroup: %w", err)
			}

			f := tabular.NewFormatter(os.Stdout)
			defer f.Flush()

			f.PrintHeader("NAME")

			for _, vm := range vms {
				f.PrintRow(vm.Name)
			}

			return nil
		}

		switches, err := client.GetVSwitches(ctx)
		if err != nil {
			return fmt.Errorf("getting vswitches: %w", err)
		}

		f := tabular.NewFormatter(os.Stdout)
		defer f.Flush()

		f.PrintHeader("SWITCH", "SWITCH TYPE", "PORTGROUP", "VLAN", "UPLINKS", "LACP", "PORTS", "USED")

		for _, sw := range switches {
			f.PrintRow(
				sw.SwitchName,
				sw.SwitchType,
				sw.Portgroup,
				sw.VLAN,
				sw.Uplinks,
				sw.LACP,
				fmt.Sprintf("%d", sw.TotalPorts),
				fmt.Sprintf("%d", sw.UsedPorts),
			)
		}

		return nil
	},
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vswitches"
	"github.com/spf13/cobra"
)

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List all virtual switches and port groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		portgroupFilter, _ := cmd.Flags().GetString("portgroup")

		c, err := loadConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), c.Timeout)
		defer cancel()

		client, err := config.NewClient(ctx, c)
		if err != nil {
			return err
		}
		defer config.Logout(ctx, client)

		if portgroupFilter != "" {
			vms, err := vswitches.GetVMsByPortgroup(ctx, client, portgroupFilter)
			if err != nil {
				return err
			}

			sort.Slice(vms, func(i, j int) bool {
				return vms[i].Name < vms[j].Name
			})

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
			for _, vm := range vms {
				ramGB := float64(vm.RAMMB) / 1024.0
				fmt.Fprintf(w, "%s\t%d\t%.1f GB\t%s\n", vm.Name, vm.VCPU, ramGB, format.Bytes(vm.StorageBytes))
			}
			w.Flush()
			return nil
		}

		switches, err := vswitches.GetSwitches(ctx, client)
		if err != nil {
			return err
		}

		sort.Slice(switches, func(i, j int) bool {
			if switches[i].Name != switches[j].Name {
				return switches[i].Name < switches[j].Name
			}
			return switches[i].Portgroup < switches[j].Portgroup
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
		for _, sw := range switches {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				sw.Name, sw.Type, sw.Portgroup, sw.VLAN, sw.Uplinks, sw.LACP, sw.TotalPorts, sw.UsedPorts)
		}
		w.Flush()

		return nil
	},
}

func init() {
	vswitchesCmd.Flags().String("portgroup", "", "filter by port group name")
}

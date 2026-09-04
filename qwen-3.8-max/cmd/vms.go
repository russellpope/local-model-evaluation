package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"vsphere-inventory/internal/format"
	"vsphere-inventory/internal/inventory"
)

func newVMsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List all virtual machines with configured vCPU, RAM, and consumed storage",
		RunE: func(cmd *cobra.Command, args []string) error {
			infos, err := inventory.ListVMs(a.ctx, a.client.Client)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
			for _, vm := range infos {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
					vm.Name, vm.VCPU, format.RAMMB(vm.RAMMB), format.Bytes(vm.CommittedBytes))
			}
			return w.Flush()
		},
	}
}

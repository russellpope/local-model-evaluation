package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/inventory"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List virtual machines with vCPU, RAM and committed storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig(cmd)
		if err != nil {
			return err
		}
		return runWithClient(cfg, func(ctx context.Context, client *vim25.Client) error {
			vms, err := inventory.ListVMs(ctx, client)
			if err != nil {
				return err
			}
			return printVMs(os.Stdout, vms)
		})
	},
}

func printVMs(w io.Writer, vms []inventory.VMInfo) error {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			vm.Name,
			vm.VCPU,
			inventory.FormatBytes(vm.RAMBytes),
			inventory.FormatBytes(vm.StorageBytes),
		)
	}
	return tw.Flush()
}

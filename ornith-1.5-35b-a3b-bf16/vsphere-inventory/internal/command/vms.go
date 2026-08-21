package command

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/example/vsphere-inventory/internal/inventory"
)

func newVMsCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "Report virtual machines in the inventory",
		Long:  "Report every virtual machine in the inventory with its configured vCPU count, memory, and committed (consumed) storage.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runVMs(cmd)
		},
	}
}

func runVMs(cmd *cobra.Command) error {
	ctx := cmd.Context()
	client, cleanup, err := connect(ctx, cmd)
	if err != nil {
		return err
	}
	defer cleanup()

	vms, err := inventory.ListVMs(ctx, client)
	if err != nil {
		return err
	}

	out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(out, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(out, "%s\t%d\t%s\t%s\n",
			vm.Name, vm.VCPUs, inventory.FormatRAMGB(vm.RAMGB), inventory.FormatBytes(vm.StorageBytes))
	}
	return out.Flush()
}

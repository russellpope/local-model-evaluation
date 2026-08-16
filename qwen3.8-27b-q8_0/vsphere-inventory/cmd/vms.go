package cmd

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines with vCPU, RAM and consumed storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, timeout, cleanup, err := connect(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		vms, err := inventory.ListVMs(ctx, client.Client)
		if err != nil {
			return opError(timeout, err)
		}

		printVMs(os.Stdout, vms)
		return nil
	},
}

func printVMs(w io.Writer, vms []inventory.VMInfo) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			vm.Name,
			vm.VCPU,
			format.FormatBytes(int64(vm.MemoryMB)<<20),
			format.FormatBytes(vm.Committed),
		)
	}
	_ = tw.Flush()
}

func init() {
	rootCmd.AddCommand(vmsCmd)
}

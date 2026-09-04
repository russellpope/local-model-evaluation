package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/vim25"

	"vsphere-inventory/internal/format"
	"vsphere-inventory/internal/inventory"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List virtual machines (vCPU, RAM, consumed storage)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return connect(cmd, func(ctx context.Context, c *vim25.Client) error {
			rows, err := inventory.ListVMs(ctx, c)
			if err != nil {
				return err
			}
			printVMs(rows)
			return nil
		})
	},
}

func printVMs(rows []inventory.VMInfo) {
	if len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "no virtual machines found")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "NAME\tVCPU\tRAM (GB)\tSTORAGE")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			r.Name, r.VCPU, format.FormatGB(r.RAMMib), format.FormatBytes(r.CommittedBytes))
	}
}

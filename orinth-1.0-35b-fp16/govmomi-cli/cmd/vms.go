package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	vim25 "github.com/vmware/govmomi/vim25"
	"govmomi-cli/internal/inventory"

	"github.com/spf13/cobra"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines with vCPU, RAM and consumed storage",
	Long:  `Enumerate every VM in the inventory and print NAME, VCPU, RAM (GiB), STORAGE (consumed).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithClient(cmd, func(ctx context.Context, cli *vim25.Client) error {
			vms, err := inventory.ListVMs(ctx, cli)
			if err != nil {
				return fmt.Errorf("listing VMs: %w", err)
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			if _, err := fmt.Fprintln(tw, "NAME\tVCPU\tRAM (GiB)\tSTORAGE"); err != nil {
				return err
			}
			for _, v := range vms {
				if _, err := fmt.Fprintf(tw, "%s\t%d\t%.1f GiB\t%s\n",
					v.Name, v.VCPUs, float64(v.MemoryMB)/1024.0, inventory.FormatBytes(v.StorageB)); err != nil {
					return err
				}
			}
			return tw.Flush()
		})
	},
}

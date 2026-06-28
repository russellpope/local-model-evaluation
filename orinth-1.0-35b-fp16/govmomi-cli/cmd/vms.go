package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"govmomi-cli/internal/inventory"

	"github.com/spf13/cobra"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines with vCPU, RAM and consumed storage",
	Long:  `Enumerate every VM in the inventory and print NAME, VCPU, RAM (GiB), STORAGE (consumed).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := getConfig()
		if err != nil {
			return err
		}

		rootCtx := cmd.Context()
		ctx, cancel := context.WithTimeout(rootCtx, cfg.Timeout)
		defer cancel()

		cli, sm, err := newClient(ctx, cfg)
		if err != nil {
			return err
		}
		defer closeClient(ctx, cli, sm)

		vms, err := inventory.ListVMs(ctx, cli)
		if err != nil {
			return fmt.Errorf("listing VMs: %w", err)
		}

		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "NAME\tVCPU\tRAM (GiB)\tSTORAGE")
		for _, v := range vms {
			fmt.Fprintf(tw, "%s\t%d\t%.1f GiB\t%s\n",
				v.Name, v.VCPUs, float64(v.MemoryMB)/1024.0, inventory.FormatBytes(v.StorageB))
		}
		return tw.Flush()
	},
}

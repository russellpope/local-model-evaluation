package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"govmomi-cli/pkg/inventory"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := GetConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
		defer cancel()

		client, err := inventory.NewClient(ctx, cfg)
		if err != nil {
			return err
		}
		// In a real app, we'd logout. For this CLI, it's fine.

		vms, err := inventory.GetVMs(ctx, client.Client)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
		for _, vm := range vms {
			fmt.Fprintf(w, "%s\t%d\t%.1fGB\t%s\n", vm.Name, vm.VCPU, vm.RAMGB, vm.Storage)
		}
		return w.Flush()
	},
}

func init() {
	RootCmd.AddCommand(vmsCmd)
}

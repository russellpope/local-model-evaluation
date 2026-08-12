package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/client"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/config"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/inventory"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/output"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List virtual machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
		defer cancel()
		c, err := client.New(ctx, cfg)
		if err != nil {
			return err
		}
		defer client.Logout(ctx, c)
		vms, err := inventory.GetVMs(ctx, c)
		if err != nil {
			return err
		}
		header := []string{"NAME", "VCPU", "RAM", "STORAGE"}
		rows := make([][]string, 0, len(vms))
		for _, vm := range vms {
			rows = append(rows, []string{vm.Name, fmt.Sprintf("%d", vm.VCPU), fmt.Sprintf("%.1f GiB", vm.RAMGiB), vm.Storage})
		}
		output.WriteTable(os.Stdout, header, rows)
		return nil
	},
}

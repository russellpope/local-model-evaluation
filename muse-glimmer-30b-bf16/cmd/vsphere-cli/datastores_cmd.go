package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/client"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/config"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/inventory"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/output"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List datastores",
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
		ds, err := inventory.GetDatastores(ctx, c)
		if err != nil {
			return err
		}
		header := []string{"NAME", "TYPE", "USED", "AVAILABLE"}
		rows := make([][]string, 0, len(ds))
		for _, d := range ds {
			rows = append(rows, []string{d.Name, d.Type, d.Used, d.Available})
		}
		output.WriteTable(os.Stdout, header, rows)
		return nil
	},
}

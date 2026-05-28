package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/client"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/config"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/ui"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/vcenter/vms"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		
		client, err := client.New(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Logout(ctx)
		
		vms, err := vms.ListVMs(ctx, client.Client)
		if err != nil {
			return fmt.Errorf("failed to list VMs: %w", err)
		}
		
		return ui.PrintVMs(cmd.OutOrStdout(), vms)
	},
}

func init() {
	rootCmd.AddCommand(vmsCmd)
}

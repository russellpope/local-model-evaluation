package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/client"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/config"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/ui"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/vcenter/datastores"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores",
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
		
		dss, err := datastores.ListDatastores(ctx, client.Client)
		if err != nil {
			return fmt.Errorf("failed to list datastores: %w", err)
		}
		
		return ui.PrintDatastores(cmd.OutOrStdout(), dss)
	},
}

func init() {
	rootCmd.AddCommand(datastoresCmd)
}

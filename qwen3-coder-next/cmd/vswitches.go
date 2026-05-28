package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/client"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/config"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/ui"
	"github.com/local-model-evaluation/qwen3-coder-next/internal/vcenter/portgroup"
)

var portgroupName string

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List all virtual switches and port groups",
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
		
		if portgroupName != "" {
			vms, err := portgroup.ListVMsByPortgroup(ctx, client.Client, portgroupName)
			if err != nil {
				return fmt.Errorf("failed to list VMs by portgroup: %w", err)
			}
			return ui.PrintVMsByPortgroup(cmd.OutOrStdout(), vms)
		}
		
		return fmt.Errorf("vswitches listing not fully implemented - use --portgroup to list VMs")
	},
}

func init() {
	vswitchesCmd.Flags().StringVar(&portgroupName, "portgroup", "", "Filter by port group name")
	rootCmd.AddCommand(vswitchesCmd)
}

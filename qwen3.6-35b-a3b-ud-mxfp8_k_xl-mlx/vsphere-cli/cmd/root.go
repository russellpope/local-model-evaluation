package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-cli/internal/config"
	"github.com/local-model-evaluation/vsphere-cli/internal/vim"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "vsphere-cli",
		Short: "vSphere Inventory CLI",
		Long:  "A CLI tool for querying vSphere inventory",
	}
	cfg *config.ViperConfig
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cfg = config.NewConfig()

	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(vswitchesCmd)

	vswitchesCmd.Flags().String("portgroup", "", "Show VMs connected to this port group")
}

func withClient(ctx context.Context, cmd *cobra.Command, args []string) (*vim.Client, context.Context, func(), error) {
	cfg.BindFlags(cmd)

	config, err := cfg.Load()
	if err != nil {
		return nil, ctx, nil, fmt.Errorf("loading config: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	client, err := vim.NewClient(ctx, config)
	if err != nil {
		cancel()
		return nil, ctx, nil, fmt.Errorf("creating client: %w", err)
	}

	logout := func() {
		client.Logout(ctx)
		cancel()
	}

	return client, ctx, logout, nil
}

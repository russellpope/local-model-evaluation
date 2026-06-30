// Package cmd implements the CLI commands for the local-model-evaluation tool.
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

// datastoresCmd defines the "datastores" subcommand, which lists all available
// datastores from the configured vCenter instance.
var datastoresCmd = &cobra.Command{
	// Use specifies the command name used on the CLI.
	Use: "datastores",
	// Short provides a one-line description shown in help output.
	Short: "List all datastores",
	// RunE is the function executed when the command is invoked. It loads the
	// configuration, creates a vCenter client, retrieves the datastore list,
	// and prints the results to the command's output.
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

// init registers the datastores command with the root command so it is
// available as a subcommand on the CLI.
func init() {
	rootCmd.AddCommand(datastoresCmd)
}

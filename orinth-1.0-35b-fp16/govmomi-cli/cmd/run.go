package cmd

import (
	"context"

	vim25 "github.com/vmware/govmomi/vim25"

	"github.com/spf13/cobra"
)

// runWithClient extracts config, sets up a timeout context, creates an
// authenticated vSphere client, and runs fn. It handles setup and teardown
// (logout) so subcommand handlers only need to implement their business logic.
//
// Errors from fn are returned as-is. Errors from setup (config, connection,
// auth) are wrapped with context. The caller is responsible for formatting
// output — this keeps output concerns (tabwriter, JSON, etc.) in the handler.
func runWithClient(cmd *cobra.Command, fn func(ctx context.Context, cli *vim25.Client) error) error {
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

	return fn(ctx, cli)
}

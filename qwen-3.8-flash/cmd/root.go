// Package cmd wires the Cobra command tree and tabwriter presentation.
package cmd

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"local-model-evaluation/govmomi-cli/internal/client"
	"local-model-evaluation/govmomi-cli/internal/config"
)

// New builds the root command and its shared configuration. It is exported
// so tests can drive command parsing without executing the binary.
func New() (*cobra.Command, *config.Config, error) {
	cfg := config.New()

	root := &cobra.Command{
		Use:           "vsphere-inventory",
		Short:         "Report vSphere virtualization inventory from a vCenter",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if err := cfg.RegisterFlags(root.PersistentFlags()); err != nil {
		return nil, nil, err
	}
	root.AddCommand(newVMsCommand(cfg))
	root.AddCommand(newDatastoresCommand(cfg))
	root.AddCommand(newVSwitchesCommand(cfg))
	return root, cfg, nil
}

// Execute runs the CLI and returns a process exit code.
func Execute(ctx context.Context, args []string, out io.Writer) int {
	root, _, err := New()
	if err != nil {
		fmt.Fprintf(out, "error: %v\n", err)
		return 1
	}
	root.SetArgs(args)
	root.SetOut(out)
	root.SetErr(out)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(out, "error: %v\n", err)
		return 1
	}
	return 0
}

// runWithClient resolves configuration, connects to vCenter within the
// configured timeout, and hands the caller an authenticated client. Logout is
// always deferred.
func runWithClient(cfg *config.Config, fn func(ctx context.Context, out io.Writer, cl *client.Client) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		path, err := cmd.Flags().GetString("config")
		if err != nil {
			return fmt.Errorf("read --config flag: %w", err)
		}
		if err := cfg.LoadConfigFile(path); err != nil {
			return err
		}
		values, err := cfg.Resolve()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), values.Timeout)
		defer cancel()

		cl, err := client.Connect(ctx, values)
		if err != nil {
			return err
		}
		defer cl.Logout()
		return fn(ctx, cmd.OutOrStdout(), cl)
	}
}

func newTable(out io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
}

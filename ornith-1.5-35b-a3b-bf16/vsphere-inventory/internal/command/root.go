// Package command wires the vsphere-inventory Cobra CLI: a root command with the
// vms, datastores, and vswitches subcommands. Connection settings come from the
// config package (Viper) and inventory retrieval comes from the inventory
// package; this layer only translates typed results into tabwriter tables.
package command

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/example/vsphere-inventory/internal/config"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
)

// logoutTimeout bounds the best-effort logout performed when a command finishes.
const logoutTimeout = 5 * time.Second

// Execute builds the root command and runs it, returning the process exit code.
func Execute() int {
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

// NewRootCommand constructs the root command and attaches the three
// subcommands along with the shared, persistent connection flags.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "vsphere-inventory",
		Short:         "Report virtualization inventory from a vCenter Server",
		Long:          "vsphere-inventory connects to a vCenter Server and reports virtual machines, datastores, and virtual switches.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.PersistentFlags().AddFlagSet(config.Flags())
	root.AddCommand(newVMsCommand(), newDatastoresCommand(), newVswitchesCommand())
	return root
}

// connect resolves the connection configuration, opens an authenticated client
// using a context derived from the configured timeout, and returns a cleanup
// function that logs out. Callers must defer the returned cleanup.
func connect(ctx context.Context, cmd *cobra.Command) (*vim25.Client, func() error, error) {
	cfg, err := config.FromFlags(cmd.Flags())
	if err != nil {
		return nil, nil, err
	}
	if cfg.URL == "" {
		return nil, nil, fmt.Errorf("no vCenter url configured; set --url, $VSPHERE_URL, or a config file")
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)

	u, err := url.Parse(cfg.URL)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("parsing url %q: %w", cfg.URL, err)
	}
	if u.User == nil {
		u.User = url.UserPassword(cfg.Username, cfg.Password)
	}

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("connecting to vCenter %q as %q: %w", u.Host, cfg.Username, err)
	}

	cleanup := func() error {
		defer cancel()
		ctx, done := context.WithTimeout(context.Background(), logoutTimeout)
		defer done()
		return client.Logout(ctx)
	}

	return client.Client, cleanup, nil
}

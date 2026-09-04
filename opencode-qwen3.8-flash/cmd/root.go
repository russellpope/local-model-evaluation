// Package cmd wires the Cobra command tree for vsphere-inventory.
package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"

	"github.com/local-model-evaluation/vsphere-inventory/internal/client"
	"github.com/local-model-evaluation/vsphere-inventory/internal/config"
)

// NewRootCommand builds the root command with shared configuration flags.
func NewRootCommand() *cobra.Command {
	cfg := config.New()

	root := &cobra.Command{
		Use:   "vsphere-inventory",
		Short: "Report vSphere virtualization inventory",
		Long: `vsphere-inventory connects to a VMware vCenter Server and reports
virtual machines, datastores, and virtual switches.

Connection settings resolve in this order (highest first):
command-line flag, environment variable (VSPHERE_*), --config YAML file,
built-in default.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			file, err := cmd.Flags().GetString("config")
			if err != nil {
				return fmt.Errorf("read --config flag: %w", err)
			}
			if err := cfg.ReadConfig(file); err != nil {
				return err
			}
			applyFlagOverrides(cfg, cmd)
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.String("url", "", "vCenter URL or host, e.g. https://vc.lab/sdk ($VSPHERE_URL)")
	pf.String("username", "", "vCenter username ($VSPHERE_USERNAME)")
	pf.String("password", "", "vCenter password ($VSPHERE_PASSWORD)")
	pf.Bool("insecure", false, "skip TLS certificate verification ($VSPHERE_INSECURE)")
	pf.Duration("timeout", config.DefaultTimeout, "overall operation timeout, e.g. 60s ($VSPHERE_TIMEOUT)")
	pf.String("config", "", "path to a YAML config file")

	root.AddCommand(newVMsCommand(cfg))
	root.AddCommand(newDatastoresCommand(cfg))
	root.AddCommand(newVSwitchesCommand(cfg))
	return root
}

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	root := NewRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(root.ErrOrStderr(), "error:", err)
		return 1
	}
	return 0
}

// applyFlagOverrides records only the flags the user actually set, so that an
// unset flag's default value never outranks environment/config-file values.
func applyFlagOverrides(cfg *config.Config, cmd *cobra.Command) {
	for _, key := range config.Keys {
		if f := cmd.Flags().Lookup(key); f != nil && f.Changed {
			cfg.SetOverride(key, f.Value.String())
		}
	}
}

// vim25 returns the underlying protocol client used by inventory fetches.
func (c *connection) vim25() *vim25.Client {
	return c.Client.Client
}

type connection struct {
	*govmomi.Client
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *cobra.Command
}

// connect resolves settings, derives a timeout context, and authenticates.
func connect(cmd *cobra.Command, cfg *config.Config) (*connection, error) {
	settings, err := cfg.Settings()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), settings.Timeout)
	cl, err := client.Connect(ctx, settings)
	if err != nil {
		cancel()
		return nil, err
	}
	return &connection{Client: cl, ctx: context.WithoutCancel(ctx), cmd: cmd, cancel: cancel}, nil
}

// close logs out of vCenter and releases the context.
func (c *connection) close() {
	if err := c.Client.Logout(c.ctx); err != nil {
		fmt.Fprintf(c.stderr(), "warning: logout failed: %v\n", err)
	}
	c.cancel()
}

func (c *connection) stderr() io.Writer {
	return c.cmd.ErrOrStderr()
}

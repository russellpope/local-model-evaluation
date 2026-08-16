// Package cmd wires the Cobra commands to the inventory logic.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"

	"github.com/local-model-evaluation/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

var (
	flagConfig   string
	flagURL      string
	flagUsername string
	flagPassword string
	flagInsecure bool
	flagTimeout  time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "Report vSphere inventory: virtual machines, datastores and virtual switches",
}

// Execute runs the root command and returns a process exit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		return 1
	}
	return 0
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagConfig, "config", "", "path to a YAML config file")
	pf.StringVar(&flagURL, "url", "", "vCenter URL or host, e.g. https://vc.lab/sdk (env VSPHERE_URL)")
	pf.StringVar(&flagUsername, "username", "", "username (env VSPHERE_USERNAME)")
	pf.StringVar(&flagPassword, "password", "", "password (env VSPHERE_PASSWORD)")
	pf.BoolVar(&flagInsecure, "insecure", false, "skip TLS certificate verification (env VSPHERE_INSECURE)")
	pf.DurationVar(&flagTimeout, "timeout", 60*time.Second, "overall operation timeout (env VSPHERE_TIMEOUT)")
}

// connect resolves the configuration (flag > env > file > default), connects
// to vCenter and returns the client plus a cleanup function that logs out
// and releases the timeout.
func connect(cmd *cobra.Command) (*govmomi.Client, context.Context, time.Duration, func(), error) {
	v, err := config.NewViper(flagConfig)
	if err != nil {
		return nil, nil, 0, nil, err
	}
	if err := config.BindFlags(v, cmd.Flags()); err != nil {
		return nil, nil, 0, nil, err
	}
	cfg, err := config.Load(v)
	if err != nil {
		return nil, nil, 0, nil, err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
	client, err := inventory.Connect(ctx, cfg)
	if err != nil {
		cancel()
		return nil, nil, 0, nil, err
	}

	cleanup := func() {
		_ = client.Logout(ctx)
		cancel()
	}
	return client, ctx, cfg.Timeout, cleanup, nil
}

// opError annotates a failed operation with timeout context when the
// operation ran out of time.
func opError(timeout time.Duration, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("operation timed out after %s: %w", timeout, err)
	}
	return err
}

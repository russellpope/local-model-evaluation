package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"

	"vsphere-inventory/internal/config"
	"vsphere-inventory/internal/vclient"
)

type app struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *govmomi.Client
}

func newRootCmd() *cobra.Command {
	a := &app{}

	root := &cobra.Command{
		Use:           "vsphere-inventory",
		Short:         "Report vSphere inventory: virtual machines, datastores, and virtual switches",
		SilenceUsage:  true,
		SilenceErrors: false,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			configFile, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			v := config.NewViper()
			if configFile != "" {
				v.SetConfigFile(configFile)
				if err := v.ReadInConfig(); err != nil {
					return fmt.Errorf("read config file %q: %w", configFile, err)
				}
			}
			if err := config.BindFlags(v, cmd.Flags()); err != nil {
				return err
			}
			cfg, err := config.Resolve(v)
			if err != nil {
				return fmt.Errorf("resolve configuration: %w", err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
			a.ctx = ctx
			a.cancel = cancel

			client, err := vclient.Connect(ctx, cfg)
			if err != nil {
				return err
			}
			a.client = client
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if a.cancel != nil {
				a.cancel()
			}
			if a.client == nil {
				return nil
			}
			logoutCtx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 10*time.Second)
			defer cancel()
			if err := a.client.Logout(logoutCtx); err != nil {
				return fmt.Errorf("log out of vCenter: %w", err)
			}
			return nil
		},
	}

	flags := root.PersistentFlags()
	flags.String("url", "", "vCenter URL or host, e.g. https://vc.lab/sdk (env VSPHERE_URL)")
	flags.String("username", "", "vCenter username (env VSPHERE_USERNAME)")
	flags.String("password", "", "vCenter password (env VSPHERE_PASSWORD)")
	flags.Bool("insecure", false, "skip TLS certificate verification (env VSPHERE_INSECURE)")
	flags.String("timeout", "", "overall operation timeout, e.g. 60s (env VSPHERE_TIMEOUT)")
	flags.String("config", "", "path to a YAML config file")

	root.AddCommand(newVMsCmd(a))
	root.AddCommand(newDatastoresCmd(a))
	root.AddCommand(newVSwitchesCmd(a))
	return root
}

func Execute() error {
	return newRootCmd().Execute()
}

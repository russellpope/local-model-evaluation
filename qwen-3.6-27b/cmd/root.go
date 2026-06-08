package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"

	"github.com/example/vsphere-inventory-cli/pkg/config"
)

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "vSphere inventory CLI tool",
	Long:  "A CLI tool to query VMware vCenter inventory: VMs, datastores, and virtual switches.",
}

var rootViper *viper.Viper

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd = newRootCmd()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vsphere-inventory",
		Short: "vSphere inventory CLI tool",
		Long:  "A CLI tool to query VMware vCenter inventory: VMs, datastores, and virtual switches.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}

	rootViper = viper.New()
	flags := cmd.PersistentFlags()

	if err := config.BindFlags(rootViper, flags); err != nil {
		fmt.Fprintf(os.Stderr, "Error binding flags: %v\n", err)
		os.Exit(1)
	}

	cmd.AddCommand(newVMsCmd())
	cmd.AddCommand(newDatastoresCmd())
	cmd.AddCommand(newSwitchesCmd())

	return cmd
}

func connectClient(ctx context.Context, cfg *config.Config) (*govmomi.Client, func(), error) {
	endpoint, err := soap.ParseURL(cfg.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("parse URL %s: %w", cfg.URL, err)
	}

	client, err := govmomi.NewClient(ctx, endpoint, cfg.Insecure)
	if err != nil {
		return nil, nil, fmt.Errorf("create client: %w", err)
	}

	if cfg.Username != "" && cfg.Password != "" {
		err = client.Login(ctx, url.UserPassword(cfg.Username, cfg.Password))
		if err != nil {
			client.Logout(ctx)
			return nil, nil, fmt.Errorf("login to %s: %w", cfg.URL, err)
		}
	}

	cleanup := func() {
		if logoutErr := client.Logout(context.Background()); logoutErr != nil {
			fmt.Fprintf(os.Stderr, "warning: logout failed: %v\n", logoutErr)
		}
	}

	return client, cleanup, nil
}

func loadConfig() (*config.Config, error) {
	return config.Load(rootViper)
}

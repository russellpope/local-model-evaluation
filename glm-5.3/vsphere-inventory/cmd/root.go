package cmd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"

	"vsphere-inventory/internal/config"
)

// state carries the resolved configuration and the connected client between
// PersistentPreRunE and the subcommands.
type state struct {
	cfg    *config.Config
	client *govmomi.Client
	cancel context.CancelFunc
	logout func()
}

var app state

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "Report vSphere inventory: virtual machines, datastores and virtual switches",
	Long: "vsphere-inventory connects to a VMware vCenter Server and reports " +
		"virtualization inventory: virtual machines, datastores (with storage " +
		"transport) and virtual switches (standard and distributed) with their " +
		"port groups.",
	SilenceUsage: true,
	Version:      "1.0.0",
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// registerConfigFlags declares the shared connection flags on cmd.
func registerConfigFlags(cmd *cobra.Command) {
	flags := cmd.PersistentFlags()
	flags.String(config.KeyURL, "", "vCenter URL or host, e.g. https://vc.lab/sdk (env VSPHERE_URL)")
	flags.String(config.KeyUsername, "", "vCenter username (env VSPHERE_USERNAME)")
	flags.String(config.KeyPassword, "", "vCenter password (env VSPHERE_PASSWORD)")
	flags.Bool(config.KeyInsecure, false, "skip TLS verification (env VSPHERE_INSECURE)")
	flags.String(config.KeyTimeout, "60s", "overall operation timeout (env VSPHERE_TIMEOUT)")
	flags.String("config", "", "path to a YAML config file providing url, username, password, insecure, timeout")
}

// bindConfig wires a viper instance to the environment and to the flags
// registered by registerConfigFlags. Viper's resolution order makes an
// explicitly set flag win over the environment, which wins over the config
// file, which wins over the built-in default.
func bindConfig(cmd *cobra.Command, v *viper.Viper) {
	v.SetEnvPrefix(config.EnvPrefix)
	v.AutomaticEnv()

	for key, def := range config.Defaults() {
		v.SetDefault(key, def)
	}

	local := cmd.Flags()
	persistent := cmd.PersistentFlags()
	for _, key := range config.Keys {
		f := local.Lookup(key)
		if f == nil {
			f = persistent.Lookup(key)
		}
		if f != nil {
			v.BindPFlag(key, f)
		}
	}
}

// readConfigFile loads an optional YAML config file into v.
func readConfigFile(v *viper.Viper, path string) error {
	if path == "" {
		return nil
	}
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config file %s: %w", path, err)
	}
	return nil
}

func init() {
	registerConfigFlags(rootCmd)

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		v := viper.New()
		bindConfig(cmd, v)

		cfgFile, _ := cmd.Flags().GetString("config")
		if err := readConfigFile(v, cfgFile); err != nil {
			return err
		}

		cfg, err := config.Load(v)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
		client, err := connect(ctx, cfg)
		if err != nil {
			cancel()
			return err
		}

		app = state{
			cfg:    cfg,
			client: client,
			cancel: cancel,
			logout: func() {
				// Logout outside the request timeout so a slow final call
				// still terminates the session cleanly.
				lctx, lcancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer lcancel()
				_ = client.Logout(lctx)
			},
		}
		return nil
	}

	rootCmd.PersistentPostRunE = func(cmd *cobra.Command, args []string) error {
		if app.logout != nil {
			app.logout()
		}
		if app.cancel != nil {
			app.cancel()
		}
		return nil
	}
}

// connect establishes an authenticated client session.
func connect(ctx context.Context, cfg *config.Config) (*govmomi.Client, error) {
	u, err := soap.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse vCenter URL %q: %w", cfg.URL, err)
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	client, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("connect and authenticate to vCenter %s: %w\n"+
			"Check the URL, username/password, and whether the server uses a self-signed certificate (--insecure)",
			cfg.URL, err)
	}
	return client, nil
}

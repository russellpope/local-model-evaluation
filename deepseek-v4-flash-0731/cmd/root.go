// Package cmd wires the Cobra command tree, Viper-backed configuration and
// tabwriter presentation around the inventory retrieval functions. All vSphere
// querying lives in the inventory package, keeping these commands thin.
package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"vint/config"
)

const (
	flagURL      = "url"
	flagUsername = "username"
	flagPassword = "password"
	flagInsecure = "insecure"
	flagTimeout  = "timeout"
	flagConfig   = "config"
)

// NewRootCmd builds the CLI with its three subcommands and the shared
// persistent connection flags.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "vint",
		Short:         "Report vCenter inventory: VMs, datastores and virtual switches",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	p := root.PersistentFlags()
	p.String(flagURL, "", "vCenter URL or host, e.g. https://vc.lab/sdk (env VSPHERE_URL)")
	p.String(flagUsername, "", "vCenter username (env VSPHERE_USERNAME)")
	p.String(flagPassword, "", "vCenter password (env VSPHERE_PASSWORD)")
	p.Bool(flagInsecure, false, "skip TLS certificate verification (env VSPHERE_INSECURE)")
	p.Duration(flagTimeout, 0, "overall operation timeout, e.g. 90s (env VSPHERE_TIMEOUT)")
	p.String(flagConfig, "", "path to a YAML config file")

	root.AddCommand(newVmsCmd(), newDatastoresCmd(), newVSwitchesCmd())
	return root
}

// resolved resolves the full connection configuration with
// flag > env > config file > default precedence.
func resolved(cmd *cobra.Command) (*config.Config, error) {
	cfgFile, _ := cmd.Flags().GetString(flagConfig)
	return config.Load(cmd.Flags(), cfgFile)
}

// requireURL fails fast with an actionable message when no endpoint is known.
func requireURL(cfg *config.Config) error {
	if cfg.URL == "" {
		return errors.New("no vCenter URL configured: pass --url, set VSPHERE_URL, or provide a config file with a url value")
	}
	return nil
}

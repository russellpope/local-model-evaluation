// Package cmd wires the CLI surface: a root command with the three
// inventory subcommands, sharing one vCenter connection configuration.
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"vsphere-inventory/internal/config"
)

var (
	flagURL      string
	flagUsername string
	flagPassword string
	flagInsecure bool
	flagTimeout  string
	flagConfig   string
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "vsphere-inventory",
		Short:         "Report virtualization inventory from a VMware vCenter Server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.StringVar(&flagURL, "url", "", "vCenter URL (env VSPHERE_URL), e.g. https://vc.lab/sdk")
	pf.StringVar(&flagUsername, "username", "", "vCenter username (env VSPHERE_USERNAME)")
	pf.StringVar(&flagPassword, "password", "", "vCenter password (env VSPHERE_PASSWORD)")
	pf.BoolVar(&flagInsecure, "insecure", false, "skip TLS certificate verification (env VSPHERE_INSECURE)")
	pf.StringVar(&flagTimeout, "timeout", "60s", "overall operation timeout (env VSPHERE_TIMEOUT)")
	pf.StringVar(&flagConfig, "config", "", "path to a YAML config file")

	root.AddCommand(newVmsCmd(), newDatastoresCmd(), newVswitchesCmd())
	return root
}

// Execute runs the CLI and returns the error for main to handle.
func Execute() error {
	return newRootCmd().Execute()
}

// loadConfig resolves connection settings with flag > env > file > default.
// Only flags the user actually passed participate as flags.
func loadConfig(cmd *cobra.Command) (*config.Config, error) {
	flags := config.FlagValues{}
	cmd.Flags().Visit(func(f *pflag.Flag) {
		switch f.Name {
		case "url", "username", "password", "insecure", "timeout":
			flags[f.Name] = f.Value.String()
		}
	})
	cfgFile, err := cmd.Flags().GetString("config")
	if err != nil {
		return nil, err
	}
	return config.Load(flags, cfgFile)
}

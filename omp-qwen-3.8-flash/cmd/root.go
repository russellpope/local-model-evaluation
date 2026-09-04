// Package cmd wires the Cobra command tree and shared connection flags.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ldh/vsphere-inventory/internal/client"
	"github.com/ldh/vsphere-inventory/internal/config"
)

const progName = "vsphere-inventory"

// NewRootCmd builds the root command with the shared connection flags bound
// into Viper. Unset flags report their (zero) default only as viper's last
// fallback, so precedence stays flag > env > config file > built-in default.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   progName,
		Short: "Report vCenter virtualization inventory",
		Long: progName + " connects to a VMware vCenter Server and reports " +
			"virtual machines, datastores, and virtual switches as plain-text tables.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true

	flags := root.PersistentFlags()
	flags.String("url", "", "vCenter URL or host, e.g. https://vc.lab/sdk (env: VSPHERE_URL)")
	flags.String("username", "", "vSphere user name (env: VSPHERE_USERNAME)")
	flags.String("password", "", "vSphere password (env: VSPHERE_PASSWORD)")
	flags.Bool("insecure", false, "skip TLS certificate verification (env: VSPHERE_INSECURE)")
	flags.Duration("timeout", 0, "overall operation timeout, e.g. 60s (env: VSPHERE_TIMEOUT)")
	flags.String("config", "", "path to a YAML configuration file")

	v := config.NewViper()
	for _, key := range []string{"url", "username", "password", "insecure", "timeout", "config"} {
		if f := flags.Lookup(key); f != nil {
			_ = v.BindPFlag(key, f)
		}
	}

	root.AddCommand(newVMsCmd(v))
	root.AddCommand(newDatastoresCmd(v))
	root.AddCommand(newVSwitchesCmd(v))
	return root
}

// Execute runs the root command and exits non-zero on failure.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, progName+": "+err.Error())
		os.Exit(1)
	}
}

// connect resolves configuration and establishes an authenticated
// connection; the caller must Close the returned Conn.
func connect(cmd *cobra.Command, v *viper.Viper) (*client.Conn, error) {
	cfg, err := config.Resolve(v)
	if err != nil {
		return nil, err
	}
	return client.New(cmd.Context(), cfg)
}

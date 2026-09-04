// Package cmd wires the cobra command tree: a root command carrying the
// shared connection flags plus the vms, datastores and vswitches
// subcommands. All rendering uses text/tabwriter.
package cmd

import (
	"github.com/spf13/cobra"

	"vsphere-inventory/internal/config"
)

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "Report vSphere inventory (VMs, datastores, vSwitches)",
	Long: `Connect to a VMware vCenter Server and report virtualization inventory.

Subcommands:
  vms         list virtual machines (vCPU, RAM, consumed storage)
  datastores  list datastores (transport type, used/available capacity)
  vswitches   list standard and distributed switches and their port groups;
              pass --portgroup <name> to list the VMs attached to a port group

Connection settings are resolved in order: command-line flag,
VSPHERE_* environment variable, YAML config file (--config), default.`,
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	config.AddFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vswitchesCmd)
}

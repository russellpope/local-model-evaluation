package main

import (
	"github.com/spf13/cobra"
	"vsphere-inventory/cmd/datastores"
	"vsphere-inventory/cmd/vms"
	"vsphere-inventory/cmd/vswitches"
)

var rootCmd = &cobra.Command{
	Use:   "vsphere-inventory",
	Short: "vSphere Inventory CLI",
	Long:  "A command-line tool to report vSphere inventory (VMs, datastores, vSwitches).",
}

func main() {
	rootCmd.AddCommand(vms.Cmd)
	rootCmd.AddCommand(datastores.Cmd)
	rootCmd.AddCommand(vswitches.Cmd)

	if err := rootCmd.Execute(); err != nil {
		cobra.CheckErr(err)
	}
}

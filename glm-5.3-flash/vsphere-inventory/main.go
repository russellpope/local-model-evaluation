// Command vsphere-inventory connects to a VMware vCenter Server and reports
// virtualization inventory: virtual machines, datastores and virtual
// switches. See the cmd package for the command tree.
package main

import (
	"os"

	"vsphere-inventory/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

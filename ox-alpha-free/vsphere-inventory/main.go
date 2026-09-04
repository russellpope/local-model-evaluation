// Command vsphere-inventory reports vSphere inventory (VMs, datastores,
// virtual switches) from a VMware vCenter Server.
package main

import (
	"fmt"
	"os"

	"vsphere-inventory/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

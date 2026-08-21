// Command vsphere-inventory is a CLI that reports virtualization inventory from
// a VMware vCenter Server. See the root command in internal/command.
package main

import (
	"os"

	"github.com/example/vsphere-inventory/internal/command"
)

func main() {
	os.Exit(command.Execute())
}

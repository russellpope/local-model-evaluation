// Command vsphere-inventory connects to a VMware vCenter Server and reports
// virtualization inventory via the vms, datastores, and vswitches subcommands.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"vsphere-inventory/cmd"
)

func main() {
	// Cancel the operation context on interrupt/terminate so an in-flight
	// vCenter call is abandoned promptly and cleanup (logout) can run.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cmd.Execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "vsphere-inventory: "+err.Error())
		os.Exit(1)
	}
}

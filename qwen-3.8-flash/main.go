// Command vsphere-inventory reports vCenter virtualization inventory.
package main

import (
	"context"
	"os"
	"os/signal"

	"local-model-evaluation/govmomi-cli/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(cmd.Execute(ctx, os.Args[1:], os.Stdout))
}

package cmd

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/spf13/cobra"

	"local-model-evaluation/govmomi-cli/internal/client"
	"local-model-evaluation/govmomi-cli/internal/config"
	"local-model-evaluation/govmomi-cli/internal/format"
	"local-model-evaluation/govmomi-cli/internal/inventory"
)

func newVMsCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines with configured CPU, memory and committed storage",
		Args:  cobra.NoArgs,
		RunE: runWithClient(cfg, func(ctx context.Context, out io.Writer, cl *client.Client) error {
			vms, err := inventory.ListVMs(ctx, cl.Vim())
			if err != nil {
				return err
			}
			renderVMs(out, vms)
			return nil
		}),
	}
}

func renderVMs(out io.Writer, vms []inventory.VMInfo) {
	w := newTable(out)
	fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		storage := "unknown"
		if vm.StorageKnown {
			storage = format.HumanBytes(vm.Storage)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			vm.Name,
			strconv.FormatInt(int64(vm.VCPU), 10),
			format.MemoryGB(vm.MemoryMB),
			storage)
	}
	_ = w.Flush()
}

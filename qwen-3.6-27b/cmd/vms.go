package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/example/vsphere-inventory-cli/pkg/config"
	"github.com/example/vsphere-inventory-cli/pkg/format"
	"github.com/example/vsphere-inventory-cli/pkg/inventory"
)

func newVMsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List all virtual machines",
		Long:  "List all virtual machines in the vCenter inventory with their resource configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVMs(cmd.Context())
		},
	}
}

func runVMs(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if err := config.Validate(cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client, cleanup, err := connectClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	vms, err := inventory.ListVMs(ctx, client)
	if err != nil {
		return fmt.Errorf("list VMs: %w", err)
	}

	sort.Slice(vms, func(i, j int) bool {
		return vms[i].Name < vms[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVCPU\tRAM (GB)\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			vm.Name,
			vm.VCPU,
			format.GBString(uint64(vm.RAMMB)),
			format.HumanBytes(vm.Storage),
		)
	}
	w.Flush()

	return nil
}

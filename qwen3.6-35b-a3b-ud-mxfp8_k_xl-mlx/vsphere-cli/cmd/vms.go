package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-cli/internal/tabular"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, ctx, logout, err := withClient(cmd.Context(), cmd, args)
		if err != nil {
			return err
		}
		defer logout()

		vms, err := client.GetVMs(ctx)
		if err != nil {
			return fmt.Errorf("getting VMs: %w", err)
		}

		f := tabular.NewFormatter(os.Stdout)
		defer f.Flush()

		f.PrintHeader("NAME", "VCPU", "RAM", "STORAGE")

		for _, vm := range vms {
			f.PrintRow(
				vm.Name,
				fmt.Sprintf("%d", vm.VCPU),
				fmt.Sprintf("%d GB", vm.RAM/(1024*1024)),
				tabular.FormatBytes(vm.Storage),
			)
		}

		return nil
	},
}

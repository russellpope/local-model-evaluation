package cmd

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"vint/format"
	"vint/inventory"
)

func newVmsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List all virtual machines (name, vCPU, RAM, committed storage)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := resolved(cmd)
			if err != nil {
				return err
			}
			if err := requireURL(cfg); err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
			defer cancel()

			c, err := inventory.Connect(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
			if err != nil {
				return err
			}
			defer c.Logout(ctx)

			vms, err := inventory.ListVMs(ctx, c.Client)
			if err != nil {
				return fmt.Errorf("vms: %w", err)
			}
			return renderVMs(cmd.OutOrStdout(), vms)
		},
	}
}

// renderVMs writes the VM table:
//
//	NAME  VCPU  RAM (GB)  STORAGE
func renderVMs(w io.Writer, vms []inventory.VMInfo) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tVCPU\tRAM (GB)\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			vm.Name, vm.CPUs, format.GigabytesFromMiB(vm.RAMMB), format.Bytes(vm.CommittedBytes))
	}
	return tw.Flush()
}

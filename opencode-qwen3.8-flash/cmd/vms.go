package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

func newVMsCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines with vCPU, RAM, and consumed storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connect(cmd, cfg)
			if err != nil {
				return err
			}
			defer conn.close()

			vms, err := inventory.FetchVMs(conn.ctx, conn.vim25())
			if err != nil {
				return fmt.Errorf("list virtual machines: %w", err)
			}
			return renderVMs(cmd.OutOrStdout(), vms)
		},
	}
}

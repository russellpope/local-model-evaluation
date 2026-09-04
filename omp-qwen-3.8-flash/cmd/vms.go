package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ldh/vsphere-inventory/internal/inventory"
)

func newVMsCmd(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines with size and consumed storage",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connect(cmd, v)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			vms, err := inventory.VMs(conn.Ctx, conn.Vim())
			if err != nil {
				return err
			}
			return printTable(stdout, "NAME\tVCPU\tRAM\tSTORAGE", vmRows(vms))
		},
	}
}

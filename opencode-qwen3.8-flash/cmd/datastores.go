package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/local-model-evaluation/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/vsphere-inventory/internal/inventory"
)

func newDatastoresCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List datastores with storage transport, used, and available capacity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connect(cmd, cfg)
			if err != nil {
				return err
			}
			defer conn.close()

			dss, err := inventory.FetchDatastores(conn.ctx, conn.vim25())
			if err != nil {
				return fmt.Errorf("list datastores: %w", err)
			}
			return renderDatastores(cmd.OutOrStdout(), dss)
		},
	}
}

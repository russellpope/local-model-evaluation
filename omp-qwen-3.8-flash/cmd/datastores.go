package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ldh/vsphere-inventory/internal/inventory"
)

func newDatastoresCmd(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List datastores with transport, used and available capacity",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			conn, err := connect(cmd, v)
			if err != nil {
				return err
			}
			defer func() { _ = conn.Close() }()

			dss, err := inventory.Datastores(conn.Ctx, conn.Vim())
			if err != nil {
				return err
			}
			return printTable(stdout, "NAME\tTYPE\tUSED\tAVAILABLE", datastoreRows(dss))
		},
	}
}

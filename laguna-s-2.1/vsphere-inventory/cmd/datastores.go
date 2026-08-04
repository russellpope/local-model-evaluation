package cmd

import (
	"context"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/datastores"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/spf13/cobra"
)

var datastoresCmd = &cobra.Command{
	Use:   "datastores",
	Short: "List all datastores",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), c.Timeout)
		defer cancel()

		client, err := config.NewClient(ctx, c)
		if err != nil {
			return err
		}
		defer config.Logout(ctx, client)

		dsList, err := datastores.GetDatastores(ctx, client)
		if err != nil {
			return err
		}

		format.RenderDatastores(cmd.OutOrStdout(), dsList)

		return nil
	},
}

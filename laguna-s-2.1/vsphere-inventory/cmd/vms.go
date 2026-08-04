package cmd

import (
	"context"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vms"
	"github.com/spf13/cobra"
)

var vmsCmd = &cobra.Command{
	Use:   "vms",
	Short: "List all virtual machines",
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

		vmsList, err := vms.GetVMs(ctx, client)
		if err != nil {
			return err
		}

		format.RenderVMs(cmd.OutOrStdout(), vmsList)

		return nil
	},
}

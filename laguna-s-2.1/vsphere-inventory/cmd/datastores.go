package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

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

		sort.Slice(dsList, func(i, j int) bool {
			return dsList[i].Name < dsList[j].Name
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
		for _, ds := range dsList {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ds.Name, ds.Type, format.Bytes(ds.UsedBytes), format.Bytes(ds.AvailableBytes))
		}
		w.Flush()

		return nil
	},
}

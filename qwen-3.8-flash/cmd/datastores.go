package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"local-model-evaluation/govmomi-cli/internal/client"
	"local-model-evaluation/govmomi-cli/internal/config"
	"local-model-evaluation/govmomi-cli/internal/format"
	"local-model-evaluation/govmomi-cli/internal/inventory"
)

func newDatastoresCommand(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List datastores with backing transport, used and available capacity",
		Args:  cobra.NoArgs,
		RunE: runWithClient(cfg, func(ctx context.Context, out io.Writer, cl *client.Client) error {
			stores, err := inventory.ListDatastores(ctx, cl.Vim())
			if err != nil {
				return err
			}
			renderDatastores(out, stores)
			return nil
		}),
	}
}

func renderDatastores(out io.Writer, stores []inventory.DatastoreInfo) {
	w := newTable(out)
	fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range stores {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			ds.Name,
			ds.Transport,
			format.HumanBytes(format.Used(ds.Capacity, ds.FreeSpace)),
			format.HumanBytes(ds.FreeSpace))
	}
	_ = w.Flush()
}

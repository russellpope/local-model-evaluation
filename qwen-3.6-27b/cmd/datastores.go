package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/example/vsphere-inventory-cli/pkg/config"
	"github.com/example/vsphere-inventory-cli/pkg/format"
	"github.com/example/vsphere-inventory-cli/pkg/inventory"
)

func newDatastoresCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List all datastores",
		Long:  "List all datastores in the vCenter inventory with their capacity and transport type.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDatastores(cmd.Context())
		},
	}
}

func runDatastores(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if err := config.Validate(cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	client, cleanup, err := connectClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	dss, err := inventory.ListDatastores(ctx, client)
	if err != nil {
		return fmt.Errorf("list datastores: %w", err)
	}

	sort.Slice(dss, func(i, j int) bool {
		return dss[i].Name < dss[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			ds.Name,
			ds.Type,
			format.HumanBytes(ds.Used),
			format.HumanBytes(ds.Available),
		)
	}
	w.Flush()

	return nil
}

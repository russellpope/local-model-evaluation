package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

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

		sort.Slice(vmsList, func(i, j int) bool {
			return vmsList[i].Name < vmsList[j].Name
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
		for _, vm := range vmsList {
			ramGB := float64(vm.RAMMB) / 1024.0
			fmt.Fprintf(w, "%s\t%d\t%.1f GB\t%s\n", vm.Name, vm.VCPU, ramGB, format.Bytes(vm.StorageBytes))
		}
		w.Flush()

		return nil
	},
}

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/config"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/vswitches"
	"github.com/spf13/cobra"
	"github.com/vmware/govmomi/find"
)

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List all virtual switches and port groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		portgroupFilter, _ := cmd.Flags().GetString("portgroup")

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

		if portgroupFilter != "" {
			vms, err := vswitches.GetVMsByPortgroup(ctx, client, portgroupFilter)
			if err != nil {
				return err
			}

			format.RenderVMs(os.Stdout, vms)
			return nil
		}

		switches, err := vswitches.GetSwitches(ctx, client)
		if err != nil {
			return err
		}

		format.RenderVSwitches(os.Stdout, switches)

		return nil
	},
}

func init() {
	vswitchesCmd.Flags().String("portgroup", "", "filter by port group name")
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	var notFoundErr *find.NotFoundError
	return errors.As(err, &notFoundErr)
}

func resolvePortgroupFromOutput(output string) (string, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("no portgroup lines in output")
	}

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("no portgroup found in output")
}

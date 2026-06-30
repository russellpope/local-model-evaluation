package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"govmomi-cli/pkg/inventory"
)

var pgName string

var switchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List all virtual switches",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := GetConfig()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
		defer cancel()

		client, err := inventory.NewClient(ctx, cfg)
		if err != nil {
			return err
		}

		if pgName != "" {
			vms, err := inventory.GetVMsInPortgroup(ctx, client.Client, pgName)
			if err != nil {
				return err
			}
			for _, vm := range vms {
				fmt.Println(vm)
			}
			return nil
		}

		switches, err := inventory.GetSwitches(ctx, client.Client)
		if err != nil {
			return err
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
		for _, s := range switches {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n", s.Switch, s.SwitchType, s.Portgroup, s.VLAN, s.Uplinks, s.LACP, s.Ports, s.Used)
		}
		return w.Flush()
	},
}

func init() {
	switchesCmd.Flags().StringVar(&pgName, "portgroup", "", "Filter by portgroup name")
	RootCmd.AddCommand(switchesCmd)
}

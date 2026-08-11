package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"vsphere-inventory/internal/config"
	"vsphere-inventory/internal/formatter"
	"vsphere-inventory/internal/vsphere"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "vsphere-inventory",
		Short: "vSphere inventory CLI tool",
		Long:  "A command-line application that connects to a VMware vCenter Server and reports virtualization inventory.",
	}

	// Add flags at root level so they apply to all subcommands
	vmsCmd := &cobra.Command{
		Use:   "vms",
		Short: "List all virtual machines",
		Long:  `Print a table of all virtual machines in the inventory with NAME, VCPU, RAM (in GB), and STORAGE (consumed/committed).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Load(cmd.Flags())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), c.Timeout)
			defer cancel()

			client, err := vsphere.Connect(ctx, c)
			if err != nil {
				return fmt.Errorf("connect to vCenter: %w", err)
			}
			defer func() {
				if err := client.Logout(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "warning: logout failed: %v\n", err)
				}
			}()

			vms, err := vsphere.ListVMs(ctx, client)
			if err != nil {
				return fmt.Errorf("list VMs: %w", err)
			}

			printVMTable(os.Stdout, vms)
			return nil
		},
	}

	datastoresCmd := &cobra.Command{
		Use:   "datastores",
		Short: "List all datastores",
		Long:  `Print a table of all datastores with NAME, TYPE (storage transport), USED, and AVAILABLE.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Load(cmd.Flags())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), c.Timeout)
			defer cancel()

			client, err := vsphere.Connect(ctx, c)
			if err != nil {
				return fmt.Errorf("connect to vCenter: %w", err)
			}
			defer func() {
				if err := client.Logout(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "warning: logout failed: %v\n", err)
				}
			}()

			dss, err := vsphere.ListDatastores(ctx, client)
			if err != nil {
				return fmt.Errorf("list datastores: %w", err)
			}

			printDatastoreTable(os.Stdout, dss)
			return nil
		},
	}

	vswitchesCmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List virtual switches and port groups",
		Long: `Print a table of all virtual switches (standard and distributed) with their port groups, VLANs, uplinks, LACP status, and port counts.
Use --portgroup <name> to list VMs connected to a specific port group instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := config.Load(cmd.Flags())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), c.Timeout)
			defer cancel()

			client, err := vsphere.Connect(ctx, c)
			if err != nil {
				return fmt.Errorf("connect to vCenter: %w", err)
			}
			defer func() {
				if err := client.Logout(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "warning: logout failed: %v\n", err)
				}
			}()

			portGroupName, _ := cmd.Flags().GetString("portgroup")
			if portGroupName != "" {
				vms, err := vsphere.ListPortGroupVMs(ctx, client, portGroupName)
				if err != nil {
					return fmt.Errorf("list port group VMs: %w", err)
				}

				printPortGroupVMTable(os.Stdout, vms)
				return nil
			}

			switches, err := vsphere.ListSwitches(ctx, client)
			if err != nil {
				return fmt.Errorf("list switches: %w", err)
			}

			printSwitchTable(os.Stdout, switches)
			return nil
		},
	}

	vswitchesCmd.Flags().String("portgroup", "", "list VMs connected to the named port group instead of switches")

	rootCmd.AddCommand(vmsCmd)
	rootCmd.AddCommand(datastoresCmd)
	rootCmd.AddCommand(vswitchesCmd)

	// Add shared flags to root command so they cascade to subcommands
	cfgFlags := pflag.NewFlagSet("vsphere-inventory", pflag.ContinueOnError)
	if err := config.BindFlags(cfgFlags); err != nil {
		fmt.Fprintf(os.Stderr, "failed to bind flags: %v\n", err)
		os.Exit(1)
	}

	rootCmd.PersistentFlags().AddFlagSet(cfgFlags)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func printVMTable(w interface{ Write([]byte) (int, error) }, vms []vsphere.VMInfo) {
	wt := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(wt, "NAME\tVCPU\tRAM (GB)\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(wt, "%s\t%d\t%s\t%s\n", vm.Name, vm.VCPU, formatter.FormatRAMGB(vm.RAMGB), vm.Storage)
	}
	if err := wt.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: flush failed: %v\n", err)
	}
}

func printDatastoreTable(w interface{ Write([]byte) (int, error) }, dss []vsphere.DatastoreInfo) {
	wt := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(wt, "NAME\tTYPE\tUSED\tAVAILABLE")
	for _, ds := range dss {
		fmt.Fprintf(wt, "%s\t%s\t%s\t%s\n", ds.Name, ds.Type, ds.Used, ds.Available)
	}
	if err := wt.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: flush failed: %v\n", err)
	}
}

func printSwitchTable(w interface{ Write([]byte) (int, error) }, switches []vsphere.SwitchInfo) {
	wt := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(wt, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, sw := range switches {
		if len(sw.PortGroups) > 0 {
			for _, pg := range sw.PortGroups {
				uplinks := ""
				if len(sw.Uplinks) > 0 {
					uplinks = sw.Uplinks[0]
					for _, u := range sw.Uplinks[1:] {
						uplinks += "," + u
					}
				}

				totalPorts := sw.TotalPorts
				usedPorts := sw.UsedPorts

				fmt.Fprintf(wt, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
					sw.SwitchName, sw.SwitchType, pg.Name, pg.VLAN, uplinks, sw.LACP, totalPorts, usedPorts)
			}
		} else {
			uplinks := ""
			if len(sw.Uplinks) > 0 {
				uplinks = sw.Uplinks[0]
				for _, u := range sw.Uplinks[1:] {
					uplinks += "," + u
				}
			}

			fmt.Fprintf(wt, "%s\t%s\t-\t-\t%s\t%s\t%d\t%d\n",
				sw.SwitchName, sw.SwitchType, uplinks, sw.LACP, sw.TotalPorts, sw.UsedPorts)
		}
	}
	if err := wt.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: flush failed: %v\n", err)
	}
}

func printPortGroupVMTable(w interface{ Write([]byte) (int, error) }, vms []vsphere.SwitchVMInfo) {
	wt := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(wt, "VM NAME\tMOREF\tPORT GROUP")
	for _, vm := range vms {
		fmt.Fprintf(wt, "%s\t%s\t%s\n", vm.Name, vm.Moref, vm.PortGroup)
	}
	if err := wt.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: flush failed: %v\n", err)
	}
}

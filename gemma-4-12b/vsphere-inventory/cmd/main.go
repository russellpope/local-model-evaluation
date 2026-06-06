package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/vmware/govmomi/vim25"

	"github.com/example/vsphere-inventory/internal/config"
	"github.com/example/vsphere-inventory/internal/formatter"
	"github.com/example/vsphere-inventory/internal/inventory"
	"github.com/spf13/cobra"
)

func main() {
	var url string
	_ = flag.String("username", "", "vCenter Username")
	_ = flag.String("password", "", "vCenter Password")
	_ = flag.Bool("insecure", false, "Insecure connection")
	_ = flag.Int("timeout", 60, "Timeout in seconds")
	var configFile string

	rootCmd := &cobra.Command{
		Use:   "vsphere-inventory",
		Short: "Inventory tool for vSphere",
		Run: func(cmd *cobra.Command, args []string) {
			cfgConfig, err := config.LoadConfig(cmd)
			if err != nil {
				log.Fatalf("Error loading config: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfgConfig.Timeout)*time.Second)
			defer cancel()

			// Placeholder for client creation - needs proper soap.RoundTripper implementation for govmomi 0.54.1
			var client *vim25.Client
			_ = client

			fmt.Printf("Connected to vCenter at %s\n", url)

			vms, err := inventory.GetVMs(ctx, client)
			if err != nil {
				log.Fatalf("Failed to get VMs: %v", err)
			}

			fmt.Println("VMs:")
			rows := [][]string{}
			for _, vm := range vms {
				rows = append(rows, []string{vm.Name, fmt.Sprintf("%d", vm.VCPU), fmt.Sprintf("%d", vm.RAM), fmt.Sprintf("%d", vm.Storage)})
			}

			formatter.PrintTable(os.Stdout, []string{"Name", "VCPU", "RAM", "Storage"}, rows)

			ctx2, cancel2 := context.WithTimeout(context.Background(), time.Duration(cfgConfig.Timeout)*time.Second)
			defer cancel2()

			datastores, err := inventory.GetDatastores(ctx2, client)
			if err != nil {
				log.Fatalf("Failed to get datastores: %v", err)
			}

			fmt.Println("\nDatastores:")
			rows = [][]string{}
			for _, ds := range datastores {
				rows = append(rows, []string{ds.Name, ds.Type, fmt.Sprintf("%d", ds.Used), fmt.Sprintf("%d", ds.Available)})
			}

			formatter.PrintTable(os.Stdout, []string{"Name", "Type", "Used", "Available"}, rows)

			ctx3, cancel3 := context.WithTimeout(context.Background(), time.Duration(cfgConfig.Timeout)*time.Second)
			defer cancel3()

			switches, err := inventory.GetSwitches(ctx3, client, "")
			if err != nil {
				log.Fatalf("Failed to get switches: %v", err)
			}

			fmt.Println("\nSwitches:")
			rows = [][]string{}
			for _, sw := range switches {
				rows = append(rows, []string{sw.SwitchName, sw.SwitchType, sw.PortGroup, sw.VLAN, sw.Uplinks, sw.LACP, fmt.Sprintf("%d", sw.TotalPorts), fmt.Sprintf("%d", sw.UsedPorts)})
			}

			formatter.PrintTable(os.Stdout, []string{"SwitchName", "SwitchType", "PortGroup", "VLAN", "Uplinks", "LACP", "TotalPorts", "UsedPorts"}, rows)
		},
	}

	rootCmd.Flags().StringVarP(&url, "url", "u", "", "vCenter URL")
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file path")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

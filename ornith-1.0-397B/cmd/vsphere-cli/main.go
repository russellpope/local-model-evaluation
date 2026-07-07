package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"

	"github.com/local-model-evaluation/vsphere-cli/internal/config"
	"github.com/local-model-evaluation/vsphere-cli/internal/formatter"
	"github.com/local-model-evaluation/vsphere-cli/internal/inventory"
)

var (
	flagURL      string
	flagUsername string
	flagPassword string
	flagInsecure bool
	flagTimeout  string
	flagConfig   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "vsphere-cli",
		Short: "vSphere inventory CLI",
	}

	rootCmd.PersistentFlags().StringVar(&flagURL, "url", "", "vCenter URL")
	rootCmd.PersistentFlags().StringVar(&flagUsername, "username", "", "vCenter username")
	rootCmd.PersistentFlags().StringVar(&flagPassword, "password", "", "vCenter password")
	rootCmd.PersistentFlags().BoolVar(&flagInsecure, "insecure", false, "skip TLS verification")
	rootCmd.PersistentFlags().StringVar(&flagTimeout, "timeout", "", "operation timeout")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "path to config file")

	rootCmd.AddCommand(vmsCmd())
	rootCmd.AddCommand(datastoresCmd())
	rootCmd.AddCommand(vswitchesCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	flagOverrides := map[string]string{
		"url":      flagURL,
		"username": flagUsername,
		"password": flagPassword,
		"timeout":  flagTimeout,
		"config":   flagConfig,
	}
	if flagInsecure {
		flagOverrides["insecure"] = "true"
	}
	return config.Load(flagOverrides, "VSPHERE")
}

func connect(ctx context.Context, cfg *config.Config) (*govmomi.Client, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	u.User = url.UserPassword(cfg.Username, cfg.Password)

	client, err := inventory.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return nil, fmt.Errorf("connect to vCenter: %w", err)
	}
	return client, nil
}

func vmsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
			defer cancel()

			client, err := connect(ctx, cfg)
			if err != nil {
				return err
			}
			defer client.Logout(ctx)

			vms, err := inventory.GetVMs(ctx, client.Client)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
			for _, vm := range vms {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
					vm.Name,
					vm.VCPU,
					formatter.FormatBytes(int64(vm.RAMMB)*1024*1024),
					formatter.FormatBytes(vm.Storage),
				)
			}
			return w.Flush()
		},
	}
}

func datastoresCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List datastores",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
			defer cancel()

			client, err := connect(ctx, cfg)
			if err != nil {
				return err
			}
			defer client.Logout(ctx)

			dss, err := inventory.GetDatastores(ctx, client.Client)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tTYPE\tUSED\tAVAILABLE")
			for _, ds := range dss {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					ds.Name,
					ds.Type,
					formatter.FormatBytes(ds.Used),
					formatter.FormatBytes(ds.Available),
				)
			}
			return w.Flush()
		},
	}
}

func vswitchesCmd() *cobra.Command {
	var portGroupName string

	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List virtual switches",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
			defer cancel()

			client, err := connect(ctx, cfg)
			if err != nil {
				return err
			}
			defer client.Logout(ctx)

			if portGroupName != "" {
				return runPortGroupLookup(ctx, client.Client, portGroupName)
			}

			switches, err := inventory.GetVSwitches(ctx, client.Client)
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
			for _, sw := range switches {
				if len(sw.PortGroups) == 0 {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
						sw.SwitchName, sw.SwitchType,
						"-", "-", sw.Uplinks, sw.LACP, sw.Ports, sw.UsedPorts,
					)
				} else {
					for i, pg := range sw.PortGroups {
						if i == 0 {
							fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
								sw.SwitchName, sw.SwitchType,
								pg.Name, pg.VLAN, sw.Uplinks, sw.LACP, sw.Ports, sw.UsedPorts,
							)
						} else {
							fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
								"", "", pg.Name, pg.VLAN, "", "", 0, 0,
							)
						}
					}
				}
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&portGroupName, "portgroup", "", "list VMs in a port group")
	return cmd
}

func runPortGroupLookup(ctx context.Context, c *vim25.Client, portGroupName string) error {
	vms, err := inventory.GetVMsByPortGroup(ctx, c, portGroupName)
	if err != nil {
		return err
	}

	if len(vms) == 0 {
		fmt.Printf("No VMs found in port group %q\n", portGroupName)
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVCPU\tRAM\tSTORAGE")
	for _, vm := range vms {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			vm.Name,
			vm.VCPU,
			formatter.FormatBytes(int64(vm.RAMMB)*1024*1024),
			formatter.FormatBytes(vm.Storage),
		)
	}
	return w.Flush()
}

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/client"
	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/config"
	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/datastores"
	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/format"
	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/vms"
	"github.com/local-model-evaluation/vsphere-inventory-cli/internal/vswitch"
)

var (
	v *viper.Viper
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func execute() error {
	v = viper.New()
	v.SetEnvPrefix(config.EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("timeout", config.DefaultTimeout)
	v.SetDefault("insecure", false)

	rootCmd := &cobra.Command{
		Use:   "vsphere-inventory",
		Short: "vSphere inventory CLI",
		Long:  "A CLI tool for reporting vSphere virtualization inventory.",
	}

	rootCmd.PersistentFlags().String("url", "", "vCenter URL (e.g. https://vc.lab/sdk)")
	rootCmd.PersistentFlags().String("username", "", "vCenter username")
	rootCmd.PersistentFlags().String("password", "", "vCenter password")
	rootCmd.PersistentFlags().Bool("insecure", false, "skip TLS verification")
	rootCmd.PersistentFlags().Duration("timeout", 60*time.Second, "overall operation timeout")
	rootCmd.PersistentFlags().String("config", "", "path to config file")

	_ = v.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
	_ = v.BindPFlag("username", rootCmd.PersistentFlags().Lookup("username"))
	_ = v.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	_ = v.BindPFlag("insecure", rootCmd.PersistentFlags().Lookup("insecure"))
	_ = v.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		configFile, _ := cmd.Flags().GetString("config")

		if configFile != "" {
			v.SetConfigFile(configFile)
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("read config file %s: %w", configFile, err)
			}
		} else {
			v.SetConfigName("config")
			v.AddConfigPath(".")
			v.AddConfigPath("$HOME/.vsphere-inventory")
			v.SetConfigType("yaml")

			if err := v.ReadInConfig(); err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
					return fmt.Errorf("read config file: %w", err)
				}
			}
		}

		return nil
	}

	rootCmd.AddCommand(vmsCmd())
	rootCmd.AddCommand(datastoresCmd())
	rootCmd.AddCommand(vswitchesCmd())

	return rootCmd.Execute()
}

func newClient(cmd *cobra.Command) (*client.Client, context.CancelFunc, error) {
	cfg, err := config.FromViper(v)
	if err != nil {
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}

	if err := config.Validate(cfg); err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)

	c, err := client.New(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return c, cancel, nil
}

func vmsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vms",
		Short: "List virtual machines",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cancel, err := newClient(cmd)
			if err != nil {
				return err
			}
			defer cancel()
			defer c.Logout(cmd.Context())

			results, err := vms.GetVMs(cmd.Context(), c.Vim25())
			if err != nil {
				return err
			}

			printVMs(results)
			return nil
		},
	}
}

func datastoresCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "datastores",
		Short: "List datastores",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cancel, err := newClient(cmd)
			if err != nil {
				return err
			}
			defer cancel()
			defer c.Logout(cmd.Context())

			results, err := datastores.GetDatastores(cmd.Context(), c.Vim25())
			if err != nil {
				return err
			}

			printDatastores(results)
			return nil
		},
	}
}

func vswitchesCmd() *cobra.Command {
	var portgroup string

	cmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List virtual switches and port groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, cancel, err := newClient(cmd)
			if err != nil {
				return err
			}
			defer cancel()
			defer c.Logout(cmd.Context())

			if portgroup != "" {
				vms, err := vswitch.GetVMsForPortGroup(cmd.Context(), c.Vim25(), portgroup)
				if err != nil {
					return err
				}
				printPortGroupVMs(vms)
				return nil
			}

			results, err := vswitch.GetSwitches(cmd.Context(), c.Vim25())
			if err != nil {
				return err
			}

			printSwitches(results)
			return nil
		},
	}

	cmd.Flags().StringVar(&portgroup, "portgroup", "", "port group name to look up connected VMs")

	return cmd
}

func printVMs(results []vms.VMInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "NAME\tVCPU\tRAM\tSTORAGE\n")
	for _, vm := range results {
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", vm.Name, vm.VCPU, format.HumanBytes(int64(vm.RAM)*format.MiB), format.HumanBytes(vm.Storage))
	}
}

func printDatastores(results []datastores.DatastoreInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "NAME\tTYPE\tUSED\tAVAILABLE\n")
	for _, ds := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", ds.Name, string(ds.Type), format.HumanBytes(ds.Used), format.HumanBytes(ds.Free))
	}
}

func printSwitches(results []vswitch.SwitchInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED\n")
	for _, sw := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			sw.SwitchName, sw.SwitchType, sw.PortGroupName, sw.VLAN, sw.Uplinks, sw.LACP, sw.Ports, sw.UsedPorts)
	}
}

func printPortGroupVMs(results []vswitch.VMInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "NAME\n")
	for _, vm := range results {
		fmt.Fprintf(w, "%s\n", vm.Name)
	}
}

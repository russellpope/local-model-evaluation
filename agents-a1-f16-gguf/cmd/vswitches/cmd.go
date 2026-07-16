package vswitches

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"vsphere-inventory/internal/config"
	"vsphere-inventory/internal/storage"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
)

var Cmd = &cobra.Command{
	Use:   "vswitches",
	Short: "List all virtual switches",
	Long:  "Print a table of all virtual switches (standard and distributed) and their port groups.",
	RunE:  run,
}

func init() {
	Cmd.Flags().String("config", "", "path to config file")
	viper.BindPFlag("config", Cmd.Flags().Lookup("config"))
	Cmd.Flags().String("portgroup", "", "list VMs connected to the named port group")
}

func run(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(viper.GetString("config"))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	urlStr := cfg.URL
	if !isURL(urlStr) {
		urlStr = "https://" + urlStr + "/sdk"
	}
	// Add credentials to URL
	if cfg.Username != "" && cfg.Password != "" {
		urlStr = strings.Replace(urlStr, "https://", "https://"+cfg.Username+":"+cfg.Password+"@", 1)
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("parsing URL: %w", err)
	}
	soapClient := soap.NewClient(u, cfg.Insecure)
	govmomiClient, err := govmomi.NewClient(ctx, u, cfg.Insecure)
	if err != nil {
		return fmt.Errorf("connecting to vCenter: %w", err)
	}
	_ = soapClient // Use soapClient if needed

	portgroupName := cmd.Flags().Lookup("portgroup").Value.String()

	if portgroupName != "" {
		vms, err := storage.GetVMsByPortGroup(ctx, govmomiClient, portgroupName)
		if err != nil {
			return err
		}
		for _, vm := range vms {
			fmt.Printf("%s\t%s\n", vm.Name, vm.PortGroup)
		}
	} else {
		switches, err := storage.GetSwitches(ctx, govmomiClient)
		if err != nil {
			return err
		}
		for _, sw := range switches {
			for _, pg := range sw.PortGroups {
				fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
					sw.Name, sw.SwitchType, pg.Name, pg.VLAN,
					fmt.Sprintf("%v", sw.Uplinks), sw.LACP, sw.TotalPorts, sw.UsedPorts)
			}
		}
	}

	return nil
}

func isURL(s string) bool {
	return len(s) > 8 && s[:8] == "https://" || len(s) > 7 && s[:7] == "http://"
}

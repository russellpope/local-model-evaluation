package vms

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
	Use:   "vms",
	Short: "List all virtual machines",
	Long:  "Print a table of all virtual machines in the inventory.",
	RunE:  run,
}

func init() {
	Cmd.Flags().String("config", "", "path to config file")
	viper.BindPFlag("config", Cmd.Flags().Lookup("config"))
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

	vms, err := storage.GetVMs(ctx, govmomiClient)
	if err != nil {
		return err
	}

	// Print VMs (need to import format package)
	// For now, just print to stdout
	for _, vm := range vms {
		fmt.Printf("%s\t%d\t%.1f GB\t%.1f GB\n", vm.Name, vm.VCPU, vm.RAMGB, vm.StorageGB)
	}

	return nil
}

func isURL(s string) bool {
	return len(s) > 8 && s[:8] == "https://" || len(s) > 7 && s[:7] == "http://"
}

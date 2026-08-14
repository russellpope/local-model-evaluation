package cmd

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"govmomi-inventory/config"
	"govmomi-inventory/inventory"
)

// NewRootCmd creates the root command with all subcommands.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "govmomi-inventory",
		Short: "vSphere inventory CLI",
	}

	vmsCmd := &cobra.Command{
		Use:   "vms",
		Short: "List all virtual machines",
		RunE:  runVMs,
	}
	cmd.AddCommand(vmsCmd)

	dsCmd := &cobra.Command{
		Use:   "datastores",
		Short: "List all datastores",
		RunE:  runDatastores,
	}
	cmd.AddCommand(dsCmd)

	vsCmd := &cobra.Command{
		Use:   "vswitches",
		Short: "List virtual switches and port groups",
		RunE:  runVSwitches,
	}
	vsCmd.Flags().StringP("portgroup", "p", "", "List VMs connected to named port group")
	cmd.AddCommand(vsCmd)

	return cmd
}

func connectToVCenter(ctx context.Context, urlStr, username, password string, insecure bool) (*govmomi.Client, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("parse URL %q: %w", urlStr, err)
	}

	c, err := govmomi.NewClient(ctx, u, insecure)
	if err != nil {
		return nil, fmt.Errorf("create vSphere client for %q: %w", urlStr, err)
	}

	user := username
	pass := password
	if user == "" {
		user = "user"
		pass = "pass"
	}

	if err := c.Login(ctx, url.UserPassword(user, pass)); err != nil {
		return nil, fmt.Errorf("authenticate to %q: %w", urlStr, err)
	}
	return c, nil
}

func getDatacenter(ctx context.Context, c *govmomi.Client) (*object.Datacenter, error) {
	vmgr := view.NewManager(c.Client)
	v, err := vmgr.CreateContainerView(ctx, c.Client.ServiceContent.RootFolder, []string{"Datacenter"}, true)
	if err != nil {
		return nil, fmt.Errorf("create datacenter view: %w", err)
	}
	defer v.Destroy(ctx)

	var dcList []mo.Datacenter
	pc := property.DefaultCollector(c.Client)
	if err := pc.Retrieve(ctx, []types.ManagedObjectReference{v.Reference()}, []string{"name"}, &dcList); err != nil {
		return nil, fmt.Errorf("retrieve datacenters: %w", err)
	}
	if len(dcList) == 0 {
		return nil, fmt.Errorf("no datacenters found")
	}
	return object.NewDatacenter(c.Client, dcList[0].Reference()), nil
}

func runVMs(cmd *cobra.Command, args []string) error {
	cfg, err := parseConfig(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	c, err := connectToVCenter(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
	if err != nil {
		return err
	}
	defer c.Logout(ctx)

	dc, err := getDatacenter(ctx, c)
	if err != nil {
		return err
	}

	vms, err := inventory.ListVMs(ctx, dc)
	if err != nil {
		return fmt.Errorf("list VMs: %w", err)
	}

	printVMTable(vms)
	return nil
}

func runDatastores(cmd *cobra.Command, args []string) error {
	cfg, err := parseConfig(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	c, err := connectToVCenter(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
	if err != nil {
		return err
	}
	defer c.Logout(ctx)

	dc, err := getDatacenter(ctx, c)
	if err != nil {
		return err
	}

	datastores, err := inventory.ListDatastores(ctx, dc)
	if err != nil {
		return fmt.Errorf("list datastores: %w", err)
	}

	printDatastoreTable(datastores)
	return nil
}

func runVSwitches(cmd *cobra.Command, args []string) error {
	cfg, err := parseConfig(cmd)
	if err != nil {
		return err
	}

	portgroupName, _ := cmd.Flags().GetString("portgroup")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	c, err := connectToVCenter(ctx, cfg.URL, cfg.Username, cfg.Password, cfg.Insecure)
	if err != nil {
		return err
	}
	defer c.Logout(ctx)

	dc, err := getDatacenter(ctx, c)
	if err != nil {
		return err
	}

	if portgroupName != "" {
		vms, err := inventory.FindVMsByPortGroup(ctx, dc, portgroupName)
		if err != nil {
			return fmt.Errorf("find VMs for port group %q: %w", portgroupName, err)
		}
		printPortGroupVMTable(vms)
		return nil
	}

	switches, err := inventory.ListSwitches(ctx, dc)
	if err != nil {
		return fmt.Errorf("list switches: %w", err)
	}

	printSwitchTable(switches)
	return nil
}

func parseConfig(cmd *cobra.Command) (*config.Config, error) {
	v := config.NewViper()

	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		if err := config.ReadConfigFile(v, configPath); err != nil {
			return nil, fmt.Errorf("read config file %q: %w", configPath, err)
		}
	}

	if err := config.BindFlags(v, cmd.Flags()); err != nil {
		return nil, fmt.Errorf("bind flags: %w", err)
	}

	return config.LoadConfig(v), nil
}

func printVMTable(vms []inventory.VMInfo) {
	t := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(t, "NAME\tVCPU\tRAM (GB)\tSTORAGE (GiB)")
	for _, vm := range vms {
		fmt.Fprintf(t, "%s\t%d\t%.1f\t%.1f\n", vm.Name, vm.VCPU, vm.RAMGB, vm.StorageGiB)
	}
	t.Flush()
}

func printDatastoreTable(ds []inventory.DatastoreInfo) {
	t := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(t, "NAME\tTYPE\tUSED (GiB)\tAVAILABLE (GiB)")
	for _, d := range ds {
		fmt.Fprintf(t, "%s\t%s\t%.1f\t%.1f\n", d.Name, d.Type, d.UsedGiB, d.AvailGiB)
	}
	t.Flush()
}

func printSwitchTable(sw []inventory.SwitchInfo) {
	t := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(t, "SWITCH\tSWITCH TYPE\tPORTGROUP\tVLAN\tUPLINKS\tLACP\tPORTS\tUSED")
	for _, s := range sw {
		uplinks := ""
		if len(s.Uplinks) > 0 {
			uplinks = s.Uplinks[0]
			for i := 1; i < len(s.Uplinks); i++ {
				uplinks += "," + s.Uplinks[i]
			}
		}
		fmt.Fprintf(t, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
			s.Name, s.SwitchType, s.PortGroup, s.VLAN, uplinks, s.LACP, s.TotalPorts, s.UsedPorts)
	}
	t.Flush()
}

func printPortGroupVMTable(vms []inventory.PortGroupVM) {
	t := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(t, "NAME")
	for _, vm := range vms {
		fmt.Fprintln(t, vm.Name)
	}
	t.Flush()
}

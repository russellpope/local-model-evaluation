package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type SwitchInfo struct {
	SwitchName    string
	SwitchType    string
	PortGroupName string
	VLAN          string
	Uplinks       string
	LACP          string
	Ports         int32
	UsedPorts     int32
}

type PortGroupVMInfo struct {
	PortGroupName string
	VMs           []string
}

func getVSwitches(ctx context.Context, client *govmomi.Client) ([]SwitchInfo, error) {
	var switchInfos []SwitchInfo

	// Get standard switches (vSwitches) from hosts
	hostSystemList, err := getAllHosts(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts: %w", err)
	}

	for _, host := range hostSystemList {
		var hostMo mo.HostSystem
		err := client.RetrieveOne(ctx, host.Reference(), []string{"name", "config.network.vswitch", "config.network.portgroup", "config.network.pnic"}, &hostMo)
		if err != nil {
			continue
		}

		for _, vswitch := range hostMo.Config.Network.Vswitch {
			vswitchName := vswitch.Name

			// Get port groups for this vSwitch
			var pgInfos []types.HostPortGroupSpec
			for _, pg := range hostMo.Config.Network.Portgroup {
				if pg.Spec.VswitchName == vswitchName {
					pgInfos = append(pgInfos, pg.Spec)
				}
			}

			// Get uplinks for this vSwitch
			uplinks := getUplinksForVSwitch(&hostMo, vswitchName)

			// Get LACP status (N/A for standard switches)
			lacp := "N/A"

			for _, pgInfo := range pgInfos {
				vlanStr := "0"
				if pgInfo.VlanId != 0 {
					vlanStr = strconv.Itoa(int(pgInfo.VlanId))
				}

				// Get port counts for standard vSwitch
				ports := int32(0)
				for _, vsw := range hostMo.Config.Network.Vswitch {
					if vsw.Name == vswitchName {
						ports = vsw.NumPorts
					}
				}

				switchInfos = append(switchInfos, SwitchInfo{
					SwitchName:    vswitchName,
					SwitchType:    "standard",
					PortGroupName: pgInfo.Name,
					VLAN:          vlanStr,
					Uplinks:       uplinks,
					LACP:          lacp,
					Ports:         ports,
					UsedPorts:     0,
				})
			}
		}
	}

	// Sort by switch name, then port group name
	sort.Slice(switchInfos, func(i, j int) bool {
		if switchInfos[i].SwitchName != switchInfos[j].SwitchName {
			return switchInfos[i].SwitchName < switchInfos[j].SwitchName
		}
		return switchInfos[i].PortGroupName < switchInfos[j].PortGroupName
	})

	// Remove duplicates
	var uniqueSwitches []SwitchInfo
	seen := make(map[string]bool)
	for _, si := range switchInfos {
		key := si.SwitchName + "|" + si.SwitchType + "|" + si.PortGroupName
		if !seen[key] {
			seen[key] = true
			uniqueSwitches = append(uniqueSwitches, si)
		}
	}

	return uniqueSwitches, nil
}

func getAllHosts(ctx context.Context, client *govmomi.Client) ([]object.HostSystem, error) {
	var hosts []object.HostSystem
	folder := object.NewRootFolder(client.Client)
	
	hostViewManager := view.NewManager(client.Client)
	v, err := hostViewManager.CreateContainerView(ctx, folder.Reference(), []string{"HostSystem"}, true)
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)

	var hostMoList []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"name"}, &hostMoList)
	if err != nil {
		return nil, err
	}

	for _, hostMo := range hostMoList {
		hosts = append(hosts, *object.NewHostSystem(client.Client, hostMo.Reference()))
	}

	return hosts, nil
}

func getUplinksForVSwitch(hostMo *mo.HostSystem, vswitchName string) string {
	for _, vswitch := range hostMo.Config.Network.Vswitch {
		if vswitch.Name == vswitchName {
			if len(vswitch.Pnic) > 0 {
				return vswitch.Pnic[0]
			}
		}
	}
	return "N/A"
}

func vswitchesPrint(switchInfos []SwitchInfo) {
	fmt.Printf("%-30s %-12s %-30s %-6s %-10s %-8s %-6s %-6s\n", "SWITCH", "SWITCH TYPE", "PORTGROUP", "VLAN", "UPLINKS", "LACP", "PORTS", "USED")
	fmt.Printf("%-30s %-12s %-30s %-6s %-10s %-8s %-6s %-6s\n", "------", "-----------", "---------", "----", "-------", "----", "-----", "----")
	for _, si := range switchInfos {
		fmt.Printf("%-30s %-12s %-30s %-6s %-10s %-8s %-6d %-6d\n", si.SwitchName, si.SwitchType, si.PortGroupName, si.VLAN, si.Uplinks, si.LACP, si.Ports, si.UsedPorts)
	}
}

var vswitchesCmd = &cobra.Command{
	Use:   "vswitches",
	Short: "Print a table of all virtual switches and port groups",
	RunE: func(cmd *cobra.Command, args []string) error {
		portGroupFlag := cmd.Flags().Lookup("portgroup")
		if portGroupFlag != nil && portGroupFlag.Changed {
			portGroupName := cmd.Flags().Lookup("portgroup").Value.String()
			return vswitchesPortGroupCmd.RunE(cmd, []string{portGroupName})
		}

		cfg := Config{
			URL:      viper.GetString("url"),
			Username: viper.GetString("username"),
			Password: viper.GetString("password"),
			Insecure: viper.GetBool("insecure"),
			Timeout:  viper.GetDuration("timeout"),
		}

		ctx := cmd.Context()
		client, err := connect(ctx, cfg)
		if err != nil {
			return err
		}
		defer client.Logout(ctx)

		switchInfos, err := getVSwitches(ctx, client)
		if err != nil {
			return err
		}

		vswitchesPrint(switchInfos)
		return nil
	},
}

func getVMsForPortGroup(ctx context.Context, client *govmomi.Client, portGroupName string) ([]PortGroupVMInfo, error) {
	var result []PortGroupVMInfo

	// Get standard port groups
	hostSystemList, err := getAllHosts(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get hosts: %w", err)
	}

	for _, host := range hostSystemList {
		var hostMo mo.HostSystem
		err := client.RetrieveOne(ctx, host.Reference(), []string{"name", "config.network.portgroup", "vm"}, &hostMo)
		if err != nil {
			continue
		}

		var pgVMs []string
		for _, pg := range hostMo.Config.Network.Portgroup {
			if pg.Spec.Name == portGroupName {
				// Get VMs connected to this port group
				for _, vmRef := range hostMo.Vm {
					var vmMo mo.VirtualMachine
					err := client.RetrieveOne(ctx, vmRef, []string{"name", "config.hardware.device"}, &vmMo)
					if err != nil {
						continue
					}

					connected := false
					for _, device := range vmMo.Config.Hardware.Device {
						if nic, ok := device.(*types.VirtualEthernetCard); ok {
							if nic.Backing != nil {
								if netBacking, ok := nic.Backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
									if netBacking.DeviceName == portGroupName {
										connected = true
										break
									}
								}
							}
						}
					}

					if connected {
						pgVMs = append(pgVMs, vmMo.Name)
					}
				}
				break
			}
		}

		if len(pgVMs) > 0 {
			result = append(result, PortGroupVMInfo{
				PortGroupName: portGroupName,
				VMs:           pgVMs,
			})
		}
	}

	return result, nil
}

func vswitchesPortGroupPrint(info PortGroupVMInfo) {
	fmt.Printf("Port Group: %s\n", info.PortGroupName)
	if len(info.VMs) == 0 {
		fmt.Println("  No VMs connected")
		return
	}
	fmt.Println("  Connected VMs:")
	for _, vm := range info.VMs {
		fmt.Printf("    - %s\n", vm)
	}
}

var vswitchesPortGroupCmd = &cobra.Command{
	Use:   "portgroup <name>",
	Short: "List VMs connected to a specific port group",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		portGroupName := args[0]

		cfg := Config{
			URL:      viper.GetString("url"),
			Username: viper.GetString("username"),
			Password: viper.GetString("password"),
			Insecure: viper.GetBool("insecure"),
			Timeout:  viper.GetDuration("timeout"),
		}

		ctx := cmd.Context()
		client, err := connect(ctx, cfg)
		if err != nil {
			return err
		}
		defer client.Logout(ctx)

		info, err := getVMsForPortGroup(ctx, client, portGroupName)
		if err != nil {
			return err
		}

		if len(info) == 0 {
			fmt.Printf("Port Group: %s\n", portGroupName)
			fmt.Println("  No VMs connected")
			return nil
		}

		for _, ig := range info {
			vswitchesPortGroupPrint(ig)
		}

		return nil
	},
}

func init() {
	vswitchesCmd.Flags().String("portgroup", "", "List VMs connected to a specific port group")
	_ = vswitchesCmd.Flags().MarkHidden("portgroup")
}

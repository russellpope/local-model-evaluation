package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

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

			// Get port counts for standard vSwitch
			ports := int32(0)
			portsAvailable := int32(0)
			for _, vsw := range hostMo.Config.Network.Vswitch {
				if vsw.Name == vswitchName {
					ports = vsw.NumPorts
					if vsw.NumPortsAvailable != 0 {
						portsAvailable = vsw.NumPortsAvailable
					}
				}
			}

			usedPorts := ports - portsAvailable

			for _, pgInfo := range pgInfos {
				vlanStr := "0"
				if pgInfo.VlanId != 0 {
					vlanStr = strconv.Itoa(int(pgInfo.VlanId))
				}

				switchInfos = append(switchInfos, SwitchInfo{
					SwitchName:    vswitchName,
					SwitchType:    "standard",
					PortGroupName: pgInfo.Name,
					VLAN:          vlanStr,
					Uplinks:       uplinks,
					LACP:          lacp,
					Ports:         ports,
					UsedPorts:     usedPorts,
				})
			}
		}
	}

	// Get distributed switches and port groups
	dvsInfos, err := getDistributedSwitches(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get distributed switches: %w", err)
	}
	switchInfos = append(switchInfos, dvsInfos...)

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

func getDistributedSwitches(ctx context.Context, client *govmomi.Client) ([]SwitchInfo, error) {
	var switchInfos []SwitchInfo

	folder := object.NewRootFolder(client.Client)
	viewManager := view.NewManager(client.Client)

	// Get DistributedVirtualSwitches
	dvsView, err := viewManager.CreateContainerView(ctx, folder.Reference(), []string{"DistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, err
	}
	defer dvsView.Destroy(ctx)

	var dvsMoList []mo.DistributedVirtualSwitch
	err = dvsView.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "config", "portgroup"}, &dvsMoList)
	if err != nil {
		return nil, err
	}

	for _, dvsMo := range dvsMoList {
		dvsName := dvsMo.Name

		lacp := "disabled"
		if dvsMo.Config != nil {
			if dvsConfig, ok := dvsMo.Config.(*types.VMwareDVSConfigInfo); ok {
				if dvsConfig.LacpApiVersion != "" {
					lacpVersion := strings.ToLower(dvsConfig.LacpApiVersion)
					if lacpVersion == "singlelag" || lacpVersion == "multiplelag" {
						lacp = "enabled"
					}
				}
			}
		}

		// Get port groups for this DVS
		for _, pgRef := range dvsMo.Portgroup {
			var pgMo mo.DistributedVirtualPortgroup
			err := client.RetrieveOne(ctx, pgRef, []string{"name", "config"}, &pgMo)
			if err != nil {
				continue
			}

			vlanStr := "0"
			// For DVS port groups, VLAN info is in the default port config
			if dvsPortSetting, ok := pgMo.Config.DefaultPortConfig.(*types.VMwareDVSPortSetting); ok {
				if dvsPortSetting.Vlan != nil {
					if vlanIdSpec, ok := dvsPortSetting.Vlan.(*types.VmwareDistributedVirtualSwitchVlanIdSpec); ok {
						vlanStr = strconv.Itoa(int(vlanIdSpec.VlanId))
					}
				}
			}

			// Get port counts for DVS
			ports := int32(0)
			for _, dvs := range dvsMoList {
				if dvs.Name == dvsName {
					ports = dvs.Summary.NumPorts
					break
				}
			}
			// For DVS, NumPortsAvailable is not in DVSSummary, so we report 0 for used ports
			usedPorts := int32(0)

			switchInfos = append(switchInfos, SwitchInfo{
				SwitchName:    dvsName,
				SwitchType:    "distributed",
				PortGroupName: pgMo.Name,
				VLAN:          vlanStr,
				Uplinks:       "N/A",
				LACP:          lacp,
				Ports:         ports,
				UsedPorts:     usedPorts,
			})
		}
	}

	return switchInfos, nil
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
				pnicKey := vswitch.Pnic[0]
				// Extract vmnicX from key like "key-vim.host.PhysicalNic-vmnic0"
				if strings.Contains(pnicKey, "PhysicalNic-") {
					parts := strings.Split(pnicKey, "PhysicalNic-")
					if len(parts) > 1 {
						return parts[1]
					}
				}
				return pnicKey
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
		portGroupName, _ := cmd.Flags().GetString("portgroup")

		cfg := Config{
			URL:      viper.GetString("url"),
			Username: viper.GetString("username"),
			Password: viper.GetString("password"),
			Insecure: viper.GetBool("insecure"),
			Timeout:  viper.GetDuration("timeout"),
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), cfg.Timeout)
		defer cancel()

		client, err := connect(ctx, cfg)
		if err != nil {
			return err
		}
		defer client.Logout(ctx)

		if portGroupName != "" {
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
		}

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

	// Get standard port groups and VMs via ContainerView
	folder := object.NewRootFolder(client.Client)
	vmViewManager := view.NewManager(client.Client)
	vmView, err := vmViewManager.CreateContainerView(ctx, folder.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create container view: %w", err)
	}
	defer vmView.Destroy(ctx)

	var vmsMo []mo.VirtualMachine
	err = vmView.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.device"}, &vmsMo)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve VMs: %w", err)
	}

	var pgVMs []string
	for _, vmMo := range vmsMo {
		connected := false
		for _, device := range vmMo.Config.Hardware.Device {
			if nic, ok := device.(types.BaseVirtualEthernetCard); ok {
				virtualNic := nic.GetVirtualEthernetCard()
				if virtualNic.Backing != nil {
					// Check standard network backing
					if netBacking, ok := virtualNic.Backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
						if netBacking.DeviceName == portGroupName {
							connected = true
							break
						}
					}
					// Check distributed virtual port backing
					if dvPortBacking, ok := virtualNic.Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
						// Resolve the portgroup key
						portgroupKey := dvPortBacking.Port.PortgroupKey
						if portgroupKey != "" {
							// Get the distributed port group name
							var dvsPgMo mo.DistributedVirtualPortgroup
							pgRef := types.ManagedObjectReference{Type: "DistributedVirtualPortgroup", Value: portgroupKey}
							err := client.RetrieveOne(ctx, pgRef, []string{"name"}, &dvsPgMo)
							if err == nil && dvsPgMo.Name == portGroupName {
								connected = true
								break
							}
						}
					}
				}
			}
		}

		if connected {
			pgVMs = append(pgVMs, vmMo.Name)
		}
	}

	if len(pgVMs) > 0 {
		result = append(result, PortGroupVMInfo{
			PortGroupName: portGroupName,
			VMs:           pgVMs,
		})
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

func init() {
	vswitchesCmd.Flags().String("portgroup", "", "List VMs connected to a specific port group")
}

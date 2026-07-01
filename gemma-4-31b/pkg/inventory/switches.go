package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type SwitchInfo struct {
	Switch     string
	SwitchType string
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	Ports      int32
	Used       int32
}

func GetSwitches(ctx context.Context, client *vim25.Client) ([]SwitchInfo, error) {
	var result []SwitchInfo

	netView, err := getNetworkView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer netView.Destroy(ctx)

	var dvsList []mo.DistributedVirtualSwitch
	err = netView.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "config.teamingPolicy"}, &dvsList)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve DVS: %w", err)
	}

	var dvpgList []mo.DistributedVirtualPortgroup
	err = netView.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "config.distributedVirtualSwitch", "config.defaultPortgroupVlan", "summary"}, &dvpgList)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve DVPGs: %w", err)
	}

	hostView, err := getHostView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer hostView.Destroy(ctx)

	var hosts []mo.HostSystem
	err = hostView.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve hosts: %w", err)
	}

	result = processSwitches(dvsList, dvpgList, hosts)

	sort.Slice(result, func(i, j int) bool {
		if result[i].Switch != result[j].Switch {
			return result[i].Switch < result[j].Switch
		}
		return result[i].Portgroup < result[j].Portgroup
	})

	return result, nil
}

func processSwitches(dvsList []mo.DistributedVirtualSwitch, dvpgList []mo.DistributedVirtualPortgroup, hosts []mo.HostSystem) []SwitchInfo {
	var result []SwitchInfo

	dvsMap := make(map[string]mo.DistributedVirtualSwitch)
	for _, dvs := range dvsList {
		dvsMap[dvs.Reference().Value] = dvs
	}

	for _, dvpg := range dvpgList {
		dvsRef := dvpg.Config.DistributedVirtualSwitch.Value
		dvs, ok := dvsMap[dvsRef]
		switchName := "Unknown"
		lacp := "N/A"
		if ok {
			switchName = dvs.Name
			if dvs.Config.TeamingPolicy != nil && dvs.Config.TeamingPolicy.LoadBalancing == "loadBalanceLACP" {
				lacp = "Enabled"
			} else {
				lacp = "Disabled"
			}
		}

		vlan := "N/A"
		if dvpg.Config.DefaultPortgroupVlan != nil {
			vlan = fmt.Sprintf("%d", dvpg.Config.DefaultPortgroupVlan.VlanId)
		}

		totalPorts := int32(0)
		usedPorts := int32(0)
		if dvpg.Summary != nil {
			totalPorts = int32(dvpg.Summary.NumPorts)
			usedPorts = int32(dvpg.Summary.NumPorts) - int32(dvpg.Summary.EffectiveNumPorts)
		}

		result = append(result, SwitchInfo{
			Switch:     switchName,
			SwitchType: "distributed",
			Portgroup:  dvpg.Name,
			VLAN:       vlan,
			Uplinks:    "N/A",
			LACP:       lacp,
			Ports:      totalPorts,
			Used:       usedPorts,
		})
	}

	for _, host := range hosts {
		if host.Config == nil || host.Config.Network == nil {
			continue
		}
		for _, vsw := range host.Config.Network.vSwitch {
			for _, pg := range vsw.Portgroup {
				vlan := "N/A"
				if pg.VlanId != nil {
					vlan = fmt.Sprintf("%d", *pg.VlanId)
				}

				result = append(result, SwitchInfo{
					Switch:     vsw.Name,
					SwitchType: "standard",
					Portgroup:  pg.Name,
					VLAN:       vlan,
					Uplinks:    "N/A",
					LACP:       "N/A",
					Ports:      0,
					Used:       0,
				})
			}
		}
	}
	return result
}

func GetVMsInPortgroup(ctx context.Context, client *vim25.Client, pgName string) ([]string, error) {
	// First, find the portgroup key for the given name
	var pgKey string
	netView, err := getNetworkView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer netView.Destroy(ctx)

	var pgs []mo.DistributedVirtualPortgroup
	err = netView.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name"}, &pgs)
	if err != nil {
		return nil, err
	}

	for _, pg := range pgs {
		if pg.Name == pgName {
			pgKey = pg.Reference().Value
			break
		}
	}

	view, err := getVMView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer view.Destroy(ctx)

	var vms []mo.VirtualMachine
	err = view.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.device"}, &vms)
	if err != nil {
		return nil, err
	}

	return processVMsInPortgroup(vms, pgKey, pgName), nil
}

func processVMsInPortgroup(vms []mo.VirtualMachine, pgKey string, pgName string) []string {
	var result []string
	for _, vm := range vms {
		if vm.Config == nil || vm.Config.Hardware == nil {
			continue
		}
		for _, dev := range vm.Config.Hardware.Device {
			if netDev, ok := dev.(*types.VirtualEthernetCard); ok {
				if b, ok := netDev.Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
					if pgKey != "" && b.Port != nil && b.Port.PortgroupKey == pgKey {
						result = append(result, vm.Name)
						break
					}
				}
				if b, ok := netDev.Backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
					if b.DeviceName == pgName {
						result = append(result, vm.Name)
						break
					}
				}
			}
		}
	}
	return result
}

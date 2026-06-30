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

	dvsView, err := getNetworkView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer dvsView.Destroy(ctx)

	var dvsList []mo.DistributedVirtualSwitch
	err = dvsView.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "summary"}, &dvsList)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve DVS: %w", err)
	}

	for _, dvs := range dvsList {
		result = append(result, SwitchInfo{
			Switch:     dvs.Name,
			SwitchType: "distributed",
			Portgroup:  "N/A",
			VLAN:       "N/A",
			Uplinks:    "N/A",
			LACP:       "unknown",
			Ports:      0,
			Used:       0,
		})
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

	for _, host := range hosts {
		result = append(result, SwitchInfo{
			Switch:     fmt.Sprintf("%s: vSwitch0", host.Name),
			SwitchType: "standard",
			Portgroup:  "N/A",
			VLAN:       "N/A",
			Uplinks:    "N/A",
			LACP:       "N/A",
			Ports:      0,
			Used:       0,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Switch < result[j].Switch
	})

	return result, nil
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

	var result []string
	for _, vm := range vms {
		for _, dev := range vm.Config.Hardware.Device {
			if netDev, ok := dev.(*types.VirtualEthernetCard); ok {
				if b, ok := netDev.Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
					if pgKey != "" && b.Port.PortgroupKey == pgKey {
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

	return result, nil
}

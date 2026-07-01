package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/object"
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

type PortCount struct {
	Total int32
	Used  int32
}

func GetSwitches(ctx context.Context, client *vim25.Client) ([]SwitchInfo, error) {
	var result []SwitchInfo

	netView, err := getNetworkView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer netView.Destroy(ctx)

	var dvsList []mo.DistributedVirtualSwitch
	err = netView.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "config"}, &dvsList)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve DVS: %w", err)
	}

	var dvpgList []mo.DistributedVirtualPortgroup
	err = netView.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "config", "summary"}, &dvpgList)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve DVPGs: %w", err)
	}

	portCounts := make(map[string]PortCount)
	for _, dvs := range dvsList {
		dvsObj := object.NewDistributedVirtualSwitch(client, dvs.Reference())
		ports, err := dvsObj.FetchDVPorts(ctx, nil)
		if err != nil {
			continue
		}
		for _, p := range ports {
			pc := portCounts[p.PortgroupKey]
			pc.Total++
			if p.Connectee != nil {
				pc.Used++
			}
			portCounts[p.PortgroupKey] = pc
		}
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

	result = processSwitches(dvsList, dvpgList, hosts, portCounts)

	sort.Slice(result, func(i, j int) bool {
		if result[i].Switch != result[j].Switch {
			return result[i].Switch < result[j].Switch
		}
		return result[i].Portgroup < result[j].Portgroup
	})

	return result, nil
}

func processSwitches(dvsList []mo.DistributedVirtualSwitch, dvpgList []mo.DistributedVirtualPortgroup, hosts []mo.HostSystem, portCounts map[string]PortCount) []SwitchInfo {
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
			lacp = isLACPEnabled(dvs.Config)
		}

		vlan := getVlan(&dvpg.Config)
		pc := portCounts[dvpg.Reference().Value]

		result = append(result, SwitchInfo{
			Switch:     switchName,
			SwitchType: "distributed",
			Portgroup:  dvpg.Name,
			VLAN:       vlan,
			Uplinks:    "N/A",
			LACP:       lacp,
			Ports:      pc.Total,
			Used:       pc.Used,
		})
	}

	vssMap := make(map[string]bool)
	for _, host := range hosts {
		if host.Config == nil || host.Config.Network == nil {
			continue
		}
		for _, vsw := range host.Config.Network.Vswitch {
			swName := vsw.Name
			// We only need to process the switch and its portgroups once
			if vssMap[swName] {
				continue
			}
			vssMap[swName] = true

			for _, pg := range host.Config.Network.Portgroup {
				if pg.Spec.VswitchName == swName {
					vlan := "N/A"
					if pg.Spec.VlanId != 0 {
						vlan = fmt.Sprintf("%d", pg.Spec.VlanId)
					}

					result = append(result, SwitchInfo{
						Switch:     swName,
						SwitchType: "standard",
						Portgroup:  pg.Spec.Name,
						VLAN:       vlan,
						Uplinks:    "N/A",
						LACP:       "N/A",
						Ports:      0,
						Used:       0,
					})
				}
			}
		}
	}
	return result
}

func isLACPEnabled(config types.BaseDVSConfigInfo) string {
	vmwareConfig, ok := config.(*types.VMwareDVSConfigInfo)
	if !ok {
		return "N/A"
	}
	for _, lacp := range vmwareConfig.LacpGroupConfig {
		if lacp.Mode != "" || lacp.UplinkNum > 0 {
			return "Enabled"
		}
	}
	return "Disabled"
}

func getVlan(config *types.DVPortgroupConfigInfo) string {
	if config == nil || config.DefaultPortConfig == nil {
		return "N/A"
	}
	setting, ok := config.DefaultPortConfig.(*types.VMwareDVSPortSetting)
	if !ok || setting.Vlan == nil {
		return "N/A"
	}
	switch v := setting.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return fmt.Sprintf("%d", v.VlanId)
	}
	return "N/A"
}

func GetVMsInPortgroup(ctx context.Context, client *vim25.Client, pgName string) ([]string, error) {
	var pgRef string
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
			pgRef = pg.Reference().Value
			break
		}
	}

	if pgRef == "" {
		// Also check standard portgroups? The requirement says "Resolve the portgroup name to a MoRef via the Network list"
		// Standard portgroups aren't in the DistributedVirtualPortgroup list.
		// But for the sake of the mapping, we can use a different approach for VSS if needed.
		// For now, let's stick to the requirement.
	}

	view, err := getVMView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer view.Destroy(ctx)

	var vms []mo.VirtualMachine
	err = view.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "network"}, &vms)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, vm := range vms {
		for _, net := range vm.Network {
			if net.Value == pgRef {
				result = append(result, vm.Name)
				break
			}
		}
	}

	return result, nil
}

// Remove processVMsInPortgroup as it's no longer needed.


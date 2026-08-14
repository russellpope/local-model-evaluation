package inventory

import (
	"context"
	"fmt"
	"strconv"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func ListSwitches(ctx context.Context, dc *object.Datacenter) ([]SwitchInfo, error) {
	var result []SwitchInfo

	hosts, err := getHosts(ctx, dc)
	if err != nil {
		return nil, fmt.Errorf("get hosts: %w", err)
	}
	for i := range hosts {
		stdSw, err := listStandardSwitches(ctx, dc.Client(), &hosts[i])
		if err != nil {
			return nil, fmt.Errorf("list standard switches for %s: %w", hosts[i].Name, err)
		}
		result = append(result, stdSw...)
	}

	dvsSw, err := listDistributedSwitches(ctx, dc)
	if err != nil {
		return nil, fmt.Errorf("list distributed switches: %w", err)
	}
	result = append(result, dvsSw...)

	sortSwitchesByTypeAndName(result)
	return result, nil
}

func listStandardSwitches(ctx context.Context, c *vim25.Client, host *mo.HostSystem) ([]SwitchInfo, error) {
	if host.Config == nil || host.Config.Network == nil {
		return nil, nil
	}

	netInfo := host.Config.Network
	var result []SwitchInfo

	vswitchUplinks := make(map[string][]string)
	for _, vsw := range netInfo.Vswitch {
		vswitchUplinks[vsw.Name] = vsw.Pnic
	}

	vswitchPGs := make(map[string][]types.HostPortGroup)
	for _, pg := range netInfo.Portgroup {
		vswitchPGs[pg.Spec.VswitchName] = append(vswitchPGs[pg.Spec.VswitchName], pg)
	}

	seen := make(map[string]bool)
	for _, vsw := range netInfo.Vswitch {
		name := vsw.Name
		uplinks := vswitchUplinks[name]
		lacp := "N/A"

		pgs := vswitchPGs[name]
		if len(pgs) == 0 {
			key := name + "/_no_pg_"
			if !seen[key] {
				seen[key] = true
				result = append(result, SwitchInfo{
					Name:       name,
					SwitchType: "standard",
					PortGroup:  "",
					VLAN:       "",
					Uplinks:    uplinks,
					LACP:       lacp,
					TotalPorts: int(vsw.NumPorts),
					UsedPorts:  int(vsw.NumPorts) - int(vsw.NumPortsAvailable),
				})
			}
		} else {
			for _, pg := range pgs {
				key := name + "/" + pg.Spec.Name
				if seen[key] {
					continue
				}
				seen[key] = true
				vlan := formatVLAN(pg.Spec.VlanId)
				result = append(result, SwitchInfo{
					Name:       name,
					SwitchType: "standard",
					PortGroup:  pg.Spec.Name,
					VLAN:       vlan,
					Uplinks:    uplinks,
					LACP:       lacp,
					TotalPorts: int(vsw.NumPorts),
					UsedPorts:  int(vsw.NumPorts) - int(vsw.NumPortsAvailable),
				})
			}
		}
	}

	return result, nil
}

func formatVLAN(vlanId int32) string {
	if vlanId == 4095 {
		return "trunk"
	}
	return strconv.Itoa(int(vlanId))
}

func listDistributedSwitches(ctx context.Context, dc *object.Datacenter) ([]SwitchInfo, error) {
	vmgr := view.NewManager(dc.Client())
	v, err := vmgr.CreateContainerView(ctx, dc.Reference(), []string{"DistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, fmt.Errorf("create DVS view: %w", err)
	}
	defer v.Destroy(ctx)

	var dvsList []mo.DistributedVirtualSwitch
	if err := v.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "config", "portgroup"}, &dvsList); err != nil {
		return nil, fmt.Errorf("retrieve DVS: %w", err)
	}

	var result []SwitchInfo
	for _, dvs := range dvsList {
		lacp := "N/A"
		if dvs.Config != nil {
			if dvsConfig, ok := dvs.Config.(*types.VMwareDVSConfigInfo); ok {
				if len(dvsConfig.LacpGroupConfig) > 0 {
					lacp = "enabled"
				} else {
					lacp = "disabled"
				}
			}
		}

		totalPorts := 0
		if dvs.Config != nil {
			if dvsConfig, ok := dvs.Config.(*types.VMwareDVSConfigInfo); ok {
				totalPorts = int(dvsConfig.NumPorts)
			}
		}

		var pgRefs []types.ManagedObjectReference
		for _, ref := range dvs.Portgroup {
			pgRefs = append(pgRefs, ref)
		}

		var pgList []mo.DistributedVirtualPortgroup
		if len(pgRefs) > 0 {
			pc := property.DefaultCollector(dc.Client())
			if err := pc.Retrieve(ctx, pgRefs, []string{"name", "config.defaultPortConfig.vlan"}, &pgList); err != nil {
				return nil, fmt.Errorf("retrieve DVS port groups: %w", err)
			}
		}

		for _, pg := range pgList {
			vlan := "0"
			result = append(result, SwitchInfo{
				Name:       dvs.Name,
				SwitchType: "distributed",
				PortGroup:  pg.Name,
				VLAN:       vlan,
				Uplinks:    nil,
				LACP:       lacp,
				TotalPorts: totalPorts,
				UsedPorts:  0,
			})
		}
	}

	return result, nil
}

func getHosts(ctx context.Context, dc *object.Datacenter) ([]mo.HostSystem, error) {
	vmgr := view.NewManager(dc.Client())
	v, err := vmgr.CreateContainerView(ctx, dc.Reference(), []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("create host view: %w", err)
	}
	defer v.Destroy(ctx)

	var hosts []mo.HostSystem
	if err := v.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts); err != nil {
		return nil, fmt.Errorf("retrieve hosts: %w", err)
	}
	return hosts, nil
}

func FindVMsByPortGroup(ctx context.Context, dc *object.Datacenter, pgName string) ([]PortGroupVM, error) {
	var result []PortGroupVM

	hosts, err := getHosts(ctx, dc)
	if err != nil {
		return nil, fmt.Errorf("get hosts: %w", err)
	}
	for i := range hosts {
		stdVMs, err := findVMsInStdPortGroup(ctx, dc.Client(), &hosts[i], pgName)
		if err != nil {
			return nil, err
		}
		result = append(result, stdVMs...)
	}

	dvsVMs, err := findVMsInDVSPortGroup(ctx, dc, pgName)
	if err != nil {
		return nil, fmt.Errorf("find DVS VMs: %w", err)
	}
	result = append(result, dvsVMs...)

	return result, nil
}

func findVMsInStdPortGroup(ctx context.Context, c *vim25.Client, host *mo.HostSystem, pgName string) ([]PortGroupVM, error) {
	if host.Config == nil || host.Config.Network == nil {
		return nil, nil
	}

	netInfo := host.Config.Network
	var pgKeys []string
	for _, pg := range netInfo.Portgroup {
		if pg.Spec.Name == pgName {
			pgKeys = append(pgKeys, pg.Spec.VswitchName+"/"+pg.Spec.Name)
		}
	}
	if len(pgKeys) == 0 {
		return nil, nil
	}

	var vmRefs []types.ManagedObjectReference
	for _, vmRef := range host.Vm {
		vmRefs = append(vmRefs, vmRef)
	}

	var vmList []mo.VirtualMachine
	if len(vmRefs) > 0 {
		pc := property.DefaultCollector(c)
		if err := pc.Retrieve(ctx, vmRefs, []string{"name", "config.hardware.device"}, &vmList); err != nil {
			return nil, fmt.Errorf("retrieve VMs: %w", err)
		}
	}

	var result []PortGroupVM
	for _, vm := range vmList {
		for _, dev := range vm.Config.Hardware.Device {
			if nic, ok := dev.(*types.VirtualEthernetCard); ok {
				if back, ok := nic.Backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
					if back.Network != nil {
						for _, pk := range pgKeys {
							if back.Network.Value == pk {
								result = append(result, PortGroupVM{Name: vm.Name})
								break
							}
						}
					}
				}
			}
		}
	}
	return result, nil
}

func findVMsInDVSPortGroup(ctx context.Context, dc *object.Datacenter, pgName string) ([]PortGroupVM, error) {
	vmgr := view.NewManager(dc.Client())
	v, err := vmgr.CreateContainerView(ctx, dc.Reference(), []string{"DistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, fmt.Errorf("create DVS view: %w", err)
	}
	defer v.Destroy(ctx)

	var dvsList []mo.DistributedVirtualSwitch
	if err := v.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "portgroup"}, &dvsList); err != nil {
		return nil, fmt.Errorf("retrieve DVS: %w", err)
	}

	var result []PortGroupVM
	for _, dvs := range dvsList {
		var pgKey string
		var pgRefs []types.ManagedObjectReference
		for _, ref := range dvs.Portgroup {
			pgRefs = append(pgRefs, ref)
		}

		var pgList []mo.DistributedVirtualPortgroup
		if len(pgRefs) > 0 {
			pc := property.DefaultCollector(dc.Client())
			if err := pc.Retrieve(ctx, pgRefs, []string{"name", "key"}, &pgList); err != nil {
				continue
			}
			for _, pg := range pgList {
				if pg.Name == pgName {
					pgKey = pg.Key
					break
				}
			}
		}
		if pgKey == "" {
			continue
		}

		vmView, err := vmgr.CreateContainerView(ctx, dc.Reference(), []string{"VirtualMachine"}, true)
		if err != nil {
			continue
		}
		var allVMs []mo.VirtualMachine
		if err := vmView.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.device"}, &allVMs); err != nil {
			vmView.Destroy(ctx)
			continue
		}
		vmView.Destroy(ctx)

		for _, vm := range allVMs {
			for _, dev := range vm.Config.Hardware.Device {
				if nic, ok := dev.(*types.VirtualEthernetCard); ok {
					if back, ok := nic.Backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
						if back.Port.PortgroupKey == pgKey {
							result = append(result, PortGroupVM{Name: vm.Name})
						}
					}
				}
			}
		}
	}
	return result, nil
}

func sortSwitchesByTypeAndName(sw []SwitchInfo) {
	for i := 1; i < len(sw); i++ {
		for j := i; j > 0 && switchLess(sw[j], sw[j-1]); j-- {
			sw[j], sw[j-1] = sw[j-1], sw[j]
		}
	}
}

func switchLess(a, b SwitchInfo) bool {
	if a.SwitchType != b.SwitchType {
		return a.SwitchType < b.SwitchType
	}
	return a.Name < b.Name
}

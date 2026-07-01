package vsphere

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// SwitchInfo holds extracted information about a virtual switch and its port groups.
type SwitchInfo struct {
	SwitchName string
	SwitchType string // "standard" or "distributed"
	PortGroups []PortGroupInfo
	TotalPorts int32
	Available  int32
	UsedPorts  int32
	LACP       string // "enabled", "disabled", or "N/A" (standard switches)
	Uplinks    []string
}

// PortGroupInfo holds information about a port group.
type PortGroupInfo struct {
	Name string
	VLAN string
}

// SwitchVMInfo holds information about VMs connected to a port group.
type SwitchVMInfo struct {
	Name      string
	Moref     string
	PortGroup string
}

// ListSwitches retrieves all virtual switches (standard and distributed) with their port groups.
func ListSwitches(ctx context.Context, client *govmomi.Client) ([]SwitchInfo, error) {
	var result []SwitchInfo

	stdSwitches, err := listStandardSwitches(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("list standard switches: %w", err)
	}
	result = append(result, stdSwitches...)

	distSwitches, err := listDistributedSwitches(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("list distributed switches: %w", err)
	}
	result = append(result, distSwitches...)

	sort.Slice(result, func(i, j int) bool {
		return result[i].SwitchName < result[j].SwitchName
	})

	return result, nil
}

// listStandardSwitches discovers standard vSwitches and their port groups.
func listStandardSwitches(ctx context.Context, client *govmomi.Client) ([]SwitchInfo, error) {
	finder := find.NewFinder(client.Client, false)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datacenters: %w", err)
	}

	seen := make(map[string]bool)
	var result []SwitchInfo

	for _, dc := range datacenters {
		hostFinder := find.NewFinder(client.Client, false)
		hostFinder.SetDatacenter(dc)
		allHosts, err := hostFinder.HostSystemList(ctx, "*")
		if err != nil {
			continue
		}

		var hostRefs []types.ManagedObjectReference
		for _, h := range allHosts {
			hostRefs = append(hostRefs, h.Reference())
		}

		if len(hostRefs) == 0 {
			continue
		}

		var hostMo []mo.HostSystem
		pc := client.PropertyCollector()
		if err := pc.Retrieve(ctx, hostRefs, []string{
			"config.network.vswitch",
			"config.network.portgroup",
		}, &hostMo); err != nil {
			continue
		}

		for _, h := range hostMo {
			if h.Config == nil || h.Config.Network == nil {
				continue
			}

			netInfo := h.Config.Network

			for _, vsw := range netInfo.Vswitch {
				key := dc.Reference().Value + "/" + vsw.Name
				if seen[key] {
					continue
				}
				seen[key] = true

				uplinks := append([]string(nil), vsw.Pnic...)

				totalPorts := vsw.NumPorts
				available := vsw.NumPortsAvailable
				usedPorts := totalPorts - available

				var portGroups []PortGroupInfo
				for _, pg := range netInfo.Portgroup {
					if pg.Spec.VswitchName != vsw.Name {
						continue
					}
					vlan := "0"
					if pg.Spec.VlanId != 0 {
						vlan = strconv.Itoa(int(pg.Spec.VlanId))
					}

					portGroups = append(portGroups, PortGroupInfo{
						Name: pg.Spec.Name,
						VLAN: vlan,
					})
				}

				result = append(result, SwitchInfo{
					SwitchName: vsw.Name,
					SwitchType: "standard",
					PortGroups: portGroups,
					TotalPorts: totalPorts,
					Available:  available,
					UsedPorts:  usedPorts,
					LACP:       "N/A",
					Uplinks:    uplinks,
				})
			}
		}
	}

	return result, nil
}

// listDistributedSwitches discovers vSphere Distributed Switches and their port groups.
func listDistributedSwitches(ctx context.Context, client *govmomi.Client) ([]SwitchInfo, error) {
	finder := find.NewFinder(client.Client, false)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datacenters: %w", err)
	}

	pc := client.PropertyCollector()
	var allDVSRefs []types.ManagedObjectReference

	for _, dc := range datacenters {
		folders, err := dc.Folders(ctx)
		if err != nil {
			continue
		}
		if folders.NetworkFolder == nil {
			continue
		}

		children, err := folders.NetworkFolder.Children(ctx)
		if err != nil {
			continue
		}

		for _, child := range children {
			if child.Reference().Type == "DistributedVirtualSwitch" {
				allDVSRefs = append(allDVSRefs, child.Reference())
			}
		}
	}

	if len(allDVSRefs) == 0 {
		return nil, nil
	}

	var dvsList []mo.DistributedVirtualSwitch
	if err := pc.Retrieve(ctx, allDVSRefs, []string{"name", "portgroup", "summary"}, &dvsList); err != nil {
		return nil, fmt.Errorf("retrieve distributed switch properties: %w", err)
	}

	var result []SwitchInfo
	for _, dvs := range dvsList {
		var portGroups []PortGroupInfo
		for _, pgRef := range dvs.Portgroup {
			var pgMo mo.DistributedVirtualPortgroup
			if err := pc.RetrieveOne(ctx, pgRef, []string{"name"}, &pgMo); err != nil {
				continue
			}

			portGroups = append(portGroups, PortGroupInfo{
				Name: pgMo.Name,
				VLAN: "0",
			})
		}

		totalPorts := dvs.Summary.NumPorts
		// Derive total ports from host members if simulator doesn't populate it.
		if totalPorts <= 0 && len(dvs.Summary.HostMember) > 0 {
			// Get port count from one of the host members' standard switch.
			for _, hostRef := range dvs.Summary.HostMember {
				var hostMo mo.HostSystem
				if err := pc.RetrieveOne(ctx, hostRef, []string{"config.network.vswitch"}, &hostMo); err != nil {
					continue
				}
				if hostMo.Config != nil && hostMo.Config.Network != nil {
					for _, vsw := range hostMo.Config.Network.Vswitch {
						if vsw.NumPorts > 0 {
							totalPorts = vsw.NumPorts * int32(len(dvs.Summary.HostMember))
							break
						}
					}
				}
				if totalPorts > 0 {
					break
				}
			}
		}

		result = append(result, SwitchInfo{
			SwitchName: dvs.Name,
			SwitchType: "distributed",
			PortGroups: portGroups,
			TotalPorts: totalPorts,
			LACP:       "N/A",
		})
	}

	return result, nil
}

// ListPortGroupVMs returns VMs connected to a specific port group (standard or distributed).
func ListPortGroupVMs(ctx context.Context, client *govmomi.Client, portGroupName string) ([]SwitchVMInfo, error) {
	finder := find.NewFinder(client.Client, false)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datacenters: %w", err)
	}

	// Try distributed port group back-reference first (fast path for DVS where vcsim may populate it).
	for _, dc := range datacenters {
		folders, err := dc.Folders(ctx)
		if err != nil {
			continue
		}
		if folders.NetworkFolder == nil {
			continue
		}
		nets, err := folders.NetworkFolder.Children(ctx)
		if err != nil {
			continue
		}
		for _, net := range nets {
			if net.Reference().Type == "DistributedVirtualPortgroup" {
				var pgMo mo.DistributedVirtualPortgroup
				pc := client.PropertyCollector()
				if err := pc.RetrieveOne(ctx, net.Reference(), []string{"name", "Vm"}, &pgMo); err != nil {
					continue
				}
				if strings.EqualFold(pgMo.Name, portGroupName) {
					vms := tryBackrefVMs(ctx, client, pgMo.Vm, portGroupName)
					if len(vms) > 0 {
						return vms, nil
					}
				}
			}
		}
	}

	// Primary method: per-VM forward-ref scan using both Network field and hardware device backing.
	// The Network field is often empty in simulators, so we also check config.hardware.device.
	for _, dc := range datacenters {
		vmFinder := find.NewFinder(client.Client, false)
		vmFinder.SetDatacenter(dc)
		vmList, err := vmFinder.VirtualMachineList(ctx, "*")
		if err != nil {
			continue
		}

		// Build DPG key->name map for distributed port group matching.
		folders, _ := dc.Folders(ctx)
		dpgKeyToName := make(map[string]string)
		if folders != nil && folders.NetworkFolder != nil {
			children, _ := folders.NetworkFolder.Children(ctx)
			for _, child := range children {
				if child.Reference().Type == "DistributedVirtualPortgroup" {
					var pgMo mo.DistributedVirtualPortgroup
					pc := client.PropertyCollector()
					if err := pc.RetrieveOne(ctx, child.Reference(), []string{"name"}, &pgMo); err == nil {
						dpgKeyToName[child.Reference().Value] = pgMo.Name
					}
				}
			}
		}

		var result []SwitchVMInfo
		for _, vm := range vmList {
			var moVM mo.VirtualMachine
			pc := client.PropertyCollector()
			if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"Network", "config.hardware.device"}, &moVM); err != nil {
				continue
			}

			// Check Network field (standard PGs where populated).
			for _, netRef := range moVM.Network {
				var netMo mo.Network
				if err := pc.RetrieveOne(ctx, netRef, []string{"name"}, &netMo); err != nil {
					continue
				}
				if strings.EqualFold(netMo.Name, portGroupName) {
					result = append(result, SwitchVMInfo{
						Name:      vm.Name(),
						Moref:     vm.Reference().Value,
						PortGroup: portGroupName,
					})
					goto done
				}
			}

			// Check hardware device backing for network adapters.
			if matchFromDevice(&moVM, portGroupName, dpgKeyToName) {
				result = append(result, SwitchVMInfo{
					Name:      vm.Name(),
					Moref:     vm.Reference().Value,
					PortGroup: portGroupName,
				})
			}

		done:
		}

		if len(result) > 0 {
			sort.Slice(result, func(i, j int) bool {
				return result[i].Name < result[j].Name
			})
			return result, nil
		}
	}

	return nil, fmt.Errorf("port group %q not found", portGroupName)
}

// matchFromDevice checks if a VM's hardware devices indicate connection to the given port group.
func matchFromDevice(moVM *mo.VirtualMachine, portGroupName string, dpgKeyToName map[string]string) bool {
	if moVM.Config == nil || moVM.Config.Hardware.Device == nil {
		return false
	}
	for _, dev := range moVM.Config.Hardware.Device {
		vdev := dev.GetVirtualDevice()
		if vdev == nil || vdev.Backing == nil {
			continue
		}
		backing := vdev.Backing
		// Standard network backing (for standard port groups).
		if nb, ok := backing.(*types.VirtualEthernetCardNetworkBackingInfo); ok {
			if nb.Network != nil && strings.EqualFold(nb.Network.Value, portGroupName) {
				return true
			}
		}
		// Distributed virtual port backing (for distributed port groups).
		if dvb, ok := backing.(*types.VirtualEthernetCardDistributedVirtualPortBackingInfo); ok {
			pgKey := dvb.Port.PortgroupKey
			if name, ok := dpgKeyToName[pgKey]; ok && strings.EqualFold(name, portGroupName) {
				return true
			}
		}
	}
	return false
}

// tryBackrefVMs resolves VMs from a DPG's Vm back-reference list.
func tryBackrefVMs(ctx context.Context, client *govmomi.Client, vmRefs []types.ManagedObjectReference, portGroupName string) []SwitchVMInfo {
	if len(vmRefs) == 0 {
		return nil
	}
	pc := client.PropertyCollector()
	var result []SwitchVMInfo
	for _, vmRef := range vmRefs {
		var moVM mo.VirtualMachine
		if err := pc.RetrieveOne(ctx, vmRef, []string{"Name"}, &moVM); err != nil {
			continue
		}
		result = append(result, SwitchVMInfo{
			Name:      moVM.Name,
			Moref:     vmRef.Value,
			PortGroup: portGroupName,
		})
	}
	if len(result) > 0 {
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
	}
	return result
}

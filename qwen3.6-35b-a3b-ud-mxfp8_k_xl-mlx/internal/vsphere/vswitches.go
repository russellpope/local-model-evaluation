package vsphere

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
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
	if err := pc.Retrieve(ctx, allDVSRefs, []string{"name", "portgroup"}, &dvsList); err != nil {
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

		result = append(result, SwitchInfo{
			SwitchName: dvs.Name,
			SwitchType: "distributed",
			PortGroups: portGroups,
			TotalPorts: dvs.Summary.NumPorts,
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

		var targetNet object.Reference
		for _, net := range nets {
			if net.Reference().Type == "Network" || net.Reference().Type == "DistributedVirtualPortgroup" {
				var netMo mo.Network
				pc := client.PropertyCollector()
				if err := pc.RetrieveOne(ctx, net.Reference(), []string{"name"}, &netMo); err != nil {
					continue
				}
				if strings.EqualFold(netMo.Name, portGroupName) {
					targetNet = object.NewNetwork(client.Client, net.Reference())
					break
				}
			}
		}

		if targetNet != nil {
			return fetchNetworkVMs(ctx, client, targetNet.Reference(), portGroupName)
		}
	}

	for _, dc := range datacenters {
		hostFinder := find.NewFinder(client.Client, false)
		hostFinder.SetDatacenter(dc)
		hosts, err := hostFinder.HostSystemList(ctx, "*")
		if err != nil {
			continue
		}

		var targetPGName string
		for _, h := range hosts {
			var hostMo mo.HostSystem
			pc := client.PropertyCollector()
			if err := pc.RetrieveOne(ctx, h.Reference(), []string{"config.network.portgroup"}, &hostMo); err != nil {
				continue
			}

			if hostMo.Config != nil && hostMo.Config.Network != nil {
				for _, pg := range hostMo.Config.Network.Portgroup {
					if strings.EqualFold(pg.Spec.Name, portGroupName) {
						targetPGName = pg.Spec.Name
						break
					}
				}
			}
			if targetPGName != "" {
				break
			}
		}

		if targetPGName == "" {
			continue
		}

		vmFinder := find.NewFinder(client.Client, false)
		vmFinder.SetDatacenter(dc)
		vmList, err := vmFinder.VirtualMachineList(ctx, "*")
		if err != nil {
			continue
		}

		var result []SwitchVMInfo
		for _, vm := range vmList {
			var moVM mo.VirtualMachine
			pc := client.PropertyCollector()
			if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"Name", "Network"}, &moVM); err != nil {
				continue
			}

			for _, netRef := range moVM.Network {
				var netMo mo.Network
				if err := pc.RetrieveOne(ctx, netRef, []string{"name"}, &netMo); err != nil {
					continue
				}
				if strings.EqualFold(netMo.Name, portGroupName) {
					result = append(result, SwitchVMInfo{
						Name:      moVM.Name,
						Moref:     vm.Reference().Value,
						PortGroup: portGroupName,
					})
					break
				}
			}
		}

		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})

		return result, nil
	}

	return nil, fmt.Errorf("port group %q not found", portGroupName)
}

// fetchNetworkVMs retrieves VMs connected to a network reference.
func fetchNetworkVMs(ctx context.Context, client *govmomi.Client, netRef types.ManagedObjectReference, portGroupName string) ([]SwitchVMInfo, error) {
	pc := client.PropertyCollector()
	var nets []mo.Network
	if err := pc.Retrieve(ctx, []types.ManagedObjectReference{netRef}, []string{"name", "Vm"}, &nets); err != nil {
		return nil, fmt.Errorf("retrieve network properties: %w", err)
	}

	if len(nets) == 0 {
		return nil, nil
	}

	var result []SwitchVMInfo
	for _, net := range nets {
		for _, vmRef := range net.Vm {
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
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

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
	var result []SwitchInfo

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

		if folders.HostFolder == nil {
			continue
		}

		// Use finder to list all host systems under this datacenter
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

		// Retrieve vSwitch info including port counts
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
				uplinks := append([]string(nil), vsw.Pnic...)

				// Get port counts from HostVirtualSwitch
				totalPorts := vsw.NumPorts
				available := vsw.NumPortsAvailable
				usedPorts := totalPorts - available

				var portGroups []PortGroupInfo
				for _, pg := range netInfo.Portgroup {
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
	rootFolder := client.Client.ServiceContent.RootFolder

	var folder mo.Folder
	pc := client.PropertyCollector()
	if err := pc.RetrieveOne(ctx, rootFolder, []string{"ChildEntity"}, &folder); err != nil {
		return nil, fmt.Errorf("retrieve root folder: %w", err)
	}

	var dvsRefs []types.ManagedObjectReference
	for _, child := range folder.ChildEntity {
		if child.Type == "DistributedVirtualSwitch" {
			dvsRefs = append(dvsRefs, child)
		}
	}

	if len(dvsRefs) == 0 {
		return nil, nil
	}

	var dvsList []mo.DistributedVirtualSwitch
	if err := pc.Retrieve(ctx, dvsRefs, []string{
		"Name",
		"Summary.NumPorts",
		"Config",
		"Portgroup",
	}, &dvsList); err != nil {
		return nil, fmt.Errorf("retrieve distributed switch properties: %w", err)
	}

	var result []SwitchInfo
	for _, dvs := range dvsList {
		totalPorts := int32(0)
		if dvs.Summary.NumPorts != 0 {
			totalPorts = dvs.Summary.NumPorts
		}

		var uplinks []string
		lacpEnabled := "N/A"
		if dvs.Config != nil {
			config := dvs.Config.GetDVSConfigInfo()
			if config != nil {
				for _, upRef := range config.UplinkPortgroup {
					var pgMo mo.DistributedVirtualPortgroup
					if err := pc.RetrieveOne(ctx, upRef, []string{"Name"}, &pgMo); err == nil {
						uplinks = append(uplinks, pgMo.Name)
					}
				}

				var portGroups []PortGroupInfo
				for _, pgRef := range dvs.Portgroup {
					var pgMo mo.DistributedVirtualPortgroup
					if err := pc.RetrieveOne(ctx, pgRef, []string{"Name", "Config.DefaultPortConfig"}, &pgMo); err != nil {
						continue
					}

					pgInfo := PortGroupInfo{
						Name: pgMo.Name,
						VLAN: "0",
					}

					if pgMo.Config.DefaultPortConfig != nil {
						portSetting := pgMo.Config.DefaultPortConfig.GetDVPortSetting()
						if portSetting != nil && portSetting.VendorSpecificConfig != nil {
							for _, entry := range portSetting.VendorSpecificConfig.KeyValue {
								if strings.Contains(entry.Key, "vlan") || strings.Contains(entry.Key, "Vlan") {
									pgInfo.VLAN = entry.OpaqueData
								}
							}
						}
					}

					portGroups = append(portGroups, pgInfo)
				}

				result = append(result, SwitchInfo{
					SwitchName: dvs.Name,
					SwitchType: "distributed",
					PortGroups: portGroups,
					TotalPorts: totalPorts,
					LACP:       lacpEnabled,
					Uplinks:    uplinks,
				})
			} else {
				result = append(result, SwitchInfo{
					SwitchName: dvs.Name,
					SwitchType: "distributed",
					TotalPorts: totalPorts,
					LACP:       lacpEnabled,
				})
			}
		} else {
			result = append(result, SwitchInfo{
				SwitchName: dvs.Name,
				SwitchType: "distributed",
				TotalPorts: totalPorts,
				LACP:       lacpEnabled,
			})
		}
	}

	return result, nil
}

// ListPortGroupVMs returns VMs connected to a specific port group (standard or distributed).
func ListPortGroupVMs(ctx context.Context, client *govmomi.Client, portGroupName string) ([]SwitchVMInfo, error) {
	var result []SwitchVMInfo

	finder := find.NewFinder(client.Client, false)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datacenters: %w", err)
	}

	var targetNet object.Reference
	for _, dc := range datacenters {
		folders, err := dc.Folders(ctx)
		if err != nil {
			continue
		}

		if folders.NetworkFolder == nil {
			continue
		}

		// Search in the datacenter's network folder
		nets, err := folders.NetworkFolder.Children(ctx)
		if err != nil {
			continue
		}

		for _, net := range nets {
			if net.Reference().Type == "Network" || net.Reference().Type == "DistributedVirtualPortgroup" {
				var netMo mo.Network
				pc := client.PropertyCollector()
				if err := pc.RetrieveOne(ctx, net.Reference(), []string{"Name"}, &netMo); err != nil {
					continue
				}
				if strings.EqualFold(netMo.Name, portGroupName) {
					targetNet = object.NewNetwork(client.Client, net.Reference())
					break
				}
			}
		}

		if targetNet != nil {
			break
		}
	}

	if targetNet == nil {
		// Also search root folder for distributed port groups that might be there
		rootFolder := client.Client.ServiceContent.RootFolder
		var folder mo.Folder
		pc := client.PropertyCollector()
		if err := pc.RetrieveOne(ctx, rootFolder, []string{"ChildEntity"}, &folder); err == nil {
			for _, child := range folder.ChildEntity {
				if child.Type == "DistributedVirtualPortgroup" {
					var netMo mo.Network
					if err := pc.RetrieveOne(ctx, child, []string{"Name"}, &netMo); err == nil {
						if strings.EqualFold(netMo.Name, portGroupName) {
							targetNet = child
							break
						}
					}
				}
			}
		}
	}

	if targetNet == nil {
		// For standard port groups, they may not be in the network folder
		// but listed in the host's network config.
		// Try to find the port group in host network configs.
		for _, dc := range datacenters {
			hostFinder := find.NewFinder(client.Client, false)
			hostFinder.SetDatacenter(dc)
			hosts, err := hostFinder.HostSystemList(ctx, "*")
			if err != nil {
				continue
			}

			for _, h := range hosts {
				var hostMo mo.HostSystem
				pc := client.PropertyCollector()
				if err := pc.RetrieveOne(ctx, h.Reference(), []string{"config.network.portgroup"}, &hostMo); err != nil {
					continue
				}

				if hostMo.Config != nil && hostMo.Config.Network != nil {
					for _, pg := range hostMo.Config.Network.Portgroup {
						if strings.EqualFold(pg.Spec.Name, portGroupName) {
							// Found the port group. For standard port groups,
							// we need to find VMs by checking their network connections.
							// Use the VM's Network field to find connected VMs.
							vmFinder := find.NewFinder(client.Client, false)
							vmFinder.SetDatacenter(dc)
							vmList, err := vmFinder.VirtualMachineList(ctx, "*")
							if err != nil {
								continue
							}

							for _, vm := range vmList {
								var moVM mo.VirtualMachine
								if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"Name", "Network"}, &moVM); err != nil {
									continue
								}

								// Check if any of the VM's networks match the port group
								// We need to resolve the network name for each network reference
								for _, netRef := range moVM.Network {
									var netMo mo.Network
									if err := pc.RetrieveOne(ctx, netRef, []string{"Name"}, &netMo); err != nil {
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
					}
				}
			}
		}

		return nil, fmt.Errorf("port group %q not found", portGroupName)
	}

	netRef := targetNet.Reference()
	pc := client.PropertyCollector()
	var nets []mo.Network
	if err := pc.Retrieve(ctx, []types.ManagedObjectReference{netRef}, []string{"Name", "Vm"}, &nets); err != nil {
		return nil, fmt.Errorf("retrieve network properties: %w", err)
	}

	if len(nets) == 0 {
		return result, nil
	}

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

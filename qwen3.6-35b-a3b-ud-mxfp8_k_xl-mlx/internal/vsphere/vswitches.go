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

	// List standard switches from hosts
	stdSwitches, err := listStandardSwitches(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("list standard switches: %w", err)
	}
	result = append(result, stdSwitches...)

	// List distributed switches
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
			continue // skip datacenters we can't access
		}

		if folders.HostFolder == nil {
			continue
		}

		hosts, err := folders.HostFolder.Children(ctx)
		if err != nil {
			continue // skip hosts we can't reach
		}

		var hostRefs []types.ManagedObjectReference
		for _, h := range hosts {
			if h.Reference().Type == "HostSystem" {
				hostRefs = append(hostRefs, h.Reference())
			}
		}

		if len(hostRefs) == 0 {
			continue
		}

		var hostMo []mo.HostSystem
		pc := client.PropertyCollector()
		if err := pc.Retrieve(ctx, hostRefs, []string{"Config.Network.Vswitch", "Config.Network.Portgroup"}, &hostMo); err != nil {
			continue // skip hosts with errors
		}

		for _, h := range hostMo {
			if h.Config == nil || h.Config.Network == nil {
				continue
			}

			netInfo := h.Config.Network

			for _, vsw := range netInfo.Vswitch {
				var uplinks []string
				for _, pnic := range vsw.Pnic {
					uplinks = append(uplinks, pnic)
				}

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
	if err := pc.Retrieve(ctx, dvsRefs, []string{"Name", "Summary.NumPorts"}, &dvsList); err != nil {
		return nil, fmt.Errorf("retrieve distributed switch properties: %w", err)
	}

	var result []SwitchInfo
	for _, dvs := range dvsList {
		totalPorts := int32(0)
		if dvs.Summary.Name != "" {
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

				if config.LinkAgreementPolicy != nil {
					if config.LinkAgreementPolicy.Policy == "loadbalance_srcdstmac" || 
					   config.LinkAgreementPolicy.Policy == "loadbalance_srcport" ||
					   config.LinkAgreementPolicy.Policy == "loadbalance_dstport" {
						lacpEnabled = "enabled"
					} else if config.LinkAgreementPolicy.Policy == "none" {
						lacpEnabled = "disabled"
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
							vs := portSetting.VendorSpecificConfig
							if vs != nil && len(vs.KeyValue) > 0 {
								for _, entry := range vs.KeyValue {
									if strings.Contains(entry.Key, "vlan") || strings.Contains(entry.Key, "Vlan") {
										pgInfo.VLAN = entry.OpaqueData
									}
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

	rootFolder := client.Client.ServiceContent.RootFolder

	var folder mo.Folder
	pc := client.PropertyCollector()
	if err := pc.RetrieveOne(ctx, rootFolder, []string{"ChildEntity"}, &folder); err != nil {
		return nil, fmt.Errorf("retrieve root folder: %w", err)
	}

	var netRefs []types.ManagedObjectReference
	for _, child := range folder.ChildEntity {
		if child.Type == "Network" || child.Type == "DistributedVirtualPortgroup" {
			netRefs = append(netRefs, child)
		}
	}

	var targetNet object.Reference
	for _, netRef := range netRefs {
		var netMo mo.Network
		if err := pc.RetrieveOne(ctx, netRef, []string{"Name"}, &netMo); err != nil {
			continue
		}
		if strings.EqualFold(netMo.Name, portGroupName) {
			targetNet = object.NewNetwork(client.Client, netRef)
			break
		}
	}

	if targetNet == nil {
		return nil, fmt.Errorf("port group %q not found", portGroupName)
	}

	var nets []mo.Network
	netRef := targetNet.Reference()
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

// parseDiskDeviceClassify maps a disk name string to its transport type.
func parseDiskDeviceClassify(diskName string) string {
	if diskName == "" {
		return "unknown"
	}

	diskUpper := strings.ToUpper(diskName)

	switch {
	case strings.HasPrefix(diskUpper, "NAA:"):
		return classifyNAADevice(diskName)
	case strings.HasPrefix(diskUpper, "T10:"):
		return classifyT10Device(diskName)
	case strings.HasPrefix(diskUpper, "VMHBA"):
		return classifyVMHBADevice(diskName)
	case strings.HasPrefix(diskUpper, "EUI:"):
		return "NVMe"
	default:
		return "unknown"
	}
}

func classifyNAADevice(diskName string) string {
	diskUpper := strings.ToUpper(diskName)

	if strings.HasPrefix(diskUpper, "NAA:EUI") {
		return "NVMe"
	}

	if strings.Contains(diskUpper, "IP:") || strings.Contains(diskUpper, "IQN:") {
		return "iSCSI"
	}

	return "FC"
}

func classifyT10Device(diskName string) string {
	diskUpper := strings.ToUpper(diskName)

	if strings.Contains(diskUpper, "NVME") || strings.Contains(diskUpper, "NVM-E") {
		return "NVMe"
	}

	if strings.Contains(diskUpper, "ISCSI") || strings.Contains(diskUpper, "IQN:") {
		return "iSCSI"
	}

	if strings.Contains(diskUpper, "FC") || strings.Contains(diskUpper, "WWN:") {
		return "FC"
	}

	return "unknown"
}

func classifyVMHBADevice(diskName string) string {
	diskUpper := strings.ToUpper(diskName)

	if strings.Contains(diskUpper, "NVME") {
		return "NVMe"
	}

	if strings.Contains(diskUpper, "ISCSI") {
		return "iSCSI"
	}

	if strings.Contains(diskUpper, "FC") || strings.Contains(diskUpper, "VMHBA") {
		return "FC"
	}

	return "unknown"
}

// extractDiskDeviceFromBacking extracts disk device info from a VM's disk backing.
func extractDiskDeviceFromBacking(backing types.BaseVirtualDeviceBackingInfo) string {
	if backing == nil {
		return ""
	}

	switch b := backing.(type) {
	case *types.VirtualDiskFlatVer2BackingInfo:
		return b.FileName
	case *types.VirtualDiskRawDiskVer2BackingInfo:
		if b.DeviceName != "" {
			return b.DeviceName
		}
		return b.DescriptorFileName
	default:
		return ""
	}
}

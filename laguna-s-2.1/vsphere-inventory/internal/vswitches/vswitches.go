package vswitches

import (
	"context"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type SwitchInfo struct {
	Name       string
	Type       string
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	TotalPorts int
	UsedPorts  int
}

type VMInfo struct {
	Name         string
	VCPU         int
	RAMMB        int
	StorageBytes int64
}

func GetSwitches(ctx context.Context, client *vim25.Client) ([]SwitchInfo, error) {
	finder := find.NewFinder(client)

	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datacenters: %w", err)
	}

	var result []SwitchInfo

	for _, dc := range dcs {
		finder.SetDatacenter(dc)

		hosts, err := finder.HostSystemList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing host systems in datacenter %s: %w", dc.Name(), err)
		}

		for _, host := range hosts {
			var hostMo mo.HostSystem
			err := host.Properties(ctx, host.Reference(), []string{
				"name",
				"config.network",
			}, &hostMo)
			if err != nil {
				return nil, fmt.Errorf("retrieving properties for host %s: %w", host.Name(), err)
			}

			if hostMo.Config == nil || hostMo.Config.Network == nil {
				continue
			}

			networkInfo := hostMo.Config.Network

			portGroupMap := make(map[string]types.HostPortGroup)
			for _, pg := range networkInfo.Portgroup {
				portGroupMap[pg.Key] = pg
			}

			for _, vsw := range networkInfo.Vswitch {
				for _, pgKey := range vsw.Portgroup {
					pg, ok := portGroupMap[pgKey]
					if !ok {
						continue
					}

					vlanID := "0"
					if pg.Spec.VlanId != 0 {
						vlanID = fmt.Sprintf("%d", pg.Spec.VlanId)
					}

					var uplinks []string
					for _, pnic := range vsw.Pnic {
						uplinks = append(uplinks, strings.TrimPrefix(pnic, "key-vim.host.PhysicalNic-"))
					}
					uplinkStr := "N/A"
					if len(uplinks) > 0 {
						uplinkStr = strings.Join(uplinks, ",")
					}

					totalPorts := int(vsw.NumPorts)
					usedPorts := totalPorts - int(vsw.NumPortsAvailable)
					if usedPorts < 0 {
						usedPorts = 0
					}

					result = append(result, SwitchInfo{
						Name:       vsw.Name,
						Type:       "standard",
						Portgroup:  pg.Spec.Name,
						VLAN:       vlanID,
						Uplinks:    uplinkStr,
						LACP:       "disabled",
						TotalPorts: totalPorts,
						UsedPorts:  usedPorts,
					})
				}
			}
		}

		networks, err := finder.NetworkList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing networks in datacenter %s: %w", dc.Name(), err)
		}

		for _, net := range networks {
			ref := net.Reference()

			switch net.(type) {
			case *object.DistributedVirtualPortgroup:
				dvpCommon := object.NewCommon(client, ref)
				var dvpMo mo.DistributedVirtualPortgroup
				err := dvpCommon.Properties(ctx, ref, []string{
					"name",
					"config",
				}, &dvpMo)
				if err != nil {
					return nil, fmt.Errorf("retrieving properties for distributed portgroup: %w", err)
				}

				vlanID := "0"
				if dvpMo.Config.DefaultPortConfig != nil {
					vlanID = resolveVlanID(dvpMo.Config.DefaultPortConfig)
				}

				totalPorts := int(dvpMo.Config.NumPorts)

				usedPorts := 0
				dvsName := "N/A"
				if dvpMo.Config.DistributedVirtualSwitch != nil {
					dvsRef := *dvpMo.Config.DistributedVirtualSwitch
					name, err := resolveDVSName(ctx, client, dvsRef)
					if err != nil {
						return nil, fmt.Errorf("resolving DVS name for portgroup %s: %w", dvpMo.Name, err)
					}
					dvsName = name
					usedPorts, err = fetchDVPortCount(ctx, client, dvsRef, dvpMo.Config.Key)
					if err != nil {
						return nil, fmt.Errorf("fetching DVPort count for portgroup %s: %w", dvpMo.Name, err)
					}
				}

				result = append(result, SwitchInfo{
					Name:       dvsName,
					Type:       "distributed",
					Portgroup:  dvpMo.Name,
					VLAN:       vlanID,
					Uplinks:    "N/A",
					LACP:       "N/A",
					TotalPorts: totalPorts,
					UsedPorts:  usedPorts,
				})
			}
		}
	}

	return result, nil
}

func resolveVlanID(portConfig types.BaseDVPortSetting) string {
	dvsPortSetting, ok := portConfig.(*types.VMwareDVSPortSetting)
	if !ok {
		return "0"
	}
	if dvsPortSetting.Vlan == nil {
		return "0"
	}
	switch vlan := dvsPortSetting.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		if vlan.VlanId == 0 {
			return "0"
		}
		return fmt.Sprintf("%d", vlan.VlanId)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(vlan.VlanId) == 0 {
			return "0"
		}
		var ranges []string
		for _, r := range vlan.VlanId {
			ranges = append(ranges, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
		return strings.Join(ranges, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		if vlan.PvlanId == 0 {
			return "0"
		}
		return fmt.Sprintf("pvlan:%d", vlan.PvlanId)
	default:
		return "0"
	}
}

func fetchDVPortCount(ctx context.Context, client *vim25.Client, dvsRef types.ManagedObjectReference, portgroupKey string) (int, error) {
	dvs := object.NewDistributedVirtualSwitch(client, dvsRef)
	criteria := &types.DistributedVirtualSwitchPortCriteria{
		PortgroupKey: []string{portgroupKey},
		Inside:       types.NewBool(true),
		Connected:    types.NewBool(true),
	}
	ports, err := dvs.FetchDVPorts(ctx, criteria)
	if err != nil {
		return 0, fmt.Errorf("fetching DVPorts for portgroup %s: %w", portgroupKey, err)
	}
	return len(ports), nil
}

func resolveDVSName(ctx context.Context, client *vim25.Client, dvsRef types.ManagedObjectReference) (string, error) {
	dvs := object.NewDistributedVirtualSwitch(client, dvsRef)
	name, err := dvs.ObjectName(ctx)
	if err != nil {
		return "", fmt.Errorf("resolving DVS name: %w", err)
	}
	return name, nil
}

func GetVMsByPortgroup(ctx context.Context, client *vim25.Client, portgroupName string) ([]VMInfo, error) {
	finder := find.NewFinder(client)

	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datacenters: %w", err)
	}

	var pgRef types.ManagedObjectReference
	var found bool
	for _, dc := range dcs {
		finder.SetDatacenter(dc)

		pg, err := finder.Network(ctx, portgroupName)
		if err == nil {
			pgRef = pg.Reference()
			found = true
			break
		}
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("finding port group %q in datacenter %s: %w", portgroupName, dc.Name(), err)
		}
	}

	if !found {
		return nil, fmt.Errorf("finding port group %q: not found", portgroupName)
	}

	var result []VMInfo
	for _, dc := range dcs {
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing virtual machines in datacenter %s: %w", dc.Name(), err)
		}

		for _, vm := range vms {
			var vmMo mo.VirtualMachine
			err := vm.Properties(ctx, vm.Reference(), []string{
				"name",
				"network",
				"config.hardware.numCPU",
				"config.hardware.memoryMB",
				"summary.storage.committed",
			}, &vmMo)
			if err != nil {
				return nil, fmt.Errorf("retrieving properties for VM %s: %w", vm.Name(), err)
			}

			connected := false
			for _, netRef := range vmMo.Network {
				if netRef == pgRef {
					connected = true
					break
				}
			}

			if !connected {
				continue
			}

			var committed int64
			if vmMo.Summary.Storage != nil {
				committed = vmMo.Summary.Storage.Committed
			}

			var vcpu int
			var ramMB int
			if vmMo.Config != nil {
				vcpu = int(vmMo.Config.Hardware.NumCPU)
				ramMB = int(vmMo.Config.Hardware.MemoryMB)
			}

			result = append(result, VMInfo{
				Name:         vmMo.Name,
				VCPU:         vcpu,
				RAMMB:        ramMB,
				StorageBytes: committed,
			})
		}
	}

	return result, nil
}

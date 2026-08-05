package vswitch

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
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

type VMInfo struct {
	Name string
}

func GetSwitches(ctx context.Context, client *vim25.Client) ([]SwitchInfo, error) {
	var results []SwitchInfo

	standard, err := getStandardSwitches(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("get standard switches: %w", err)
	}
	results = append(results, standard...)

	distributed, err := getDistributedSwitches(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("get distributed switches: %w", err)
	}
	results = append(results, distributed...)

	sort.Slice(results, func(i, j int) bool {
		if results[i].SwitchName != results[j].SwitchName {
			return results[i].SwitchName < results[j].SwitchName
		}
		return results[i].PortGroupName < results[j].PortGroupName
	})

	return results, nil
}

func getStandardSwitches(ctx context.Context, client *vim25.Client) ([]SwitchInfo, error) {
	finder := find.NewFinder(client)

	hostRefs, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}

	var results []SwitchInfo

	for _, hostRef := range hostRefs {
		var hostMo mo.HostSystem

		props := []string{"name", "config.network"}

		if err := hostRef.Properties(ctx, hostRef.Reference(), props, &hostMo); err != nil {
			return nil, fmt.Errorf("retrieve host %q properties: %w", hostRef.Name(), err)
		}

		if hostMo.Config == nil || hostMo.Config.Network == nil {
			continue
		}

		network := hostMo.Config.Network

		for _, vswitch := range network.Vswitch {
			uplinks := strings.Join(vswitch.Pnic, ", ")
			if uplinks == "" {
				uplinks = "N/A"
			}

			usedPorts := int32(0)
			if vswitch.NumPortsAvailable >= 0 && vswitch.NumPorts >= vswitch.NumPortsAvailable {
				usedPorts = vswitch.NumPorts - vswitch.NumPortsAvailable
			}

			for _, pg := range network.Portgroup {
				if pg.Spec.VswitchName != vswitch.Name {
					continue
				}

				vlan := formatStandardVLAN(pg.Spec.VlanId)

				results = append(results, SwitchInfo{
					SwitchName:    vswitch.Name,
					SwitchType:    "standard",
					PortGroupName: pg.Spec.Name,
					VLAN:          vlan,
					Uplinks:       uplinks,
					LACP:          "N/A",
					Ports:         vswitch.NumPorts,
					UsedPorts:     usedPorts,
				})
			}
		}
	}

	return results, nil
}

func getDistributedSwitches(ctx context.Context, client *vim25.Client) ([]SwitchInfo, error) {
	m := view.NewManager(client)

	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"DistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for DVS: %w", err)
	}
	defer v.Destroy(ctx)

	var dvsList []mo.DistributedVirtualSwitch
	if err := v.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "config", "portgroup"}, &dvsList); err != nil {
		return nil, fmt.Errorf("retrieve distributed virtual switches: %w", err)
	}

	var results []SwitchInfo

	for _, dvsMo := range dvsList {
		lacp := "disabled"
		var ports int32
		var uplinkName string

		if dvsMo.Config != nil {
			config := dvsMo.Config.GetDVSConfigInfo()
			ports = config.NumPorts

			if vmwareConfig, ok := dvsMo.Config.(*types.VMwareDVSConfigInfo); ok {
				if len(vmwareConfig.LacpGroupConfig) > 0 {
					lacp = "enabled"
				}

				if len(vmwareConfig.UplinkPortgroup) > 0 {
					uplinkName = vmwareConfig.UplinkPortgroup[0].Value
				}
			}
		}

		if uplinkName == "" {
			uplinkName = "N/A"
		}

		for _, pgRef := range dvsMo.Portgroup {
			var pgMo mo.DistributedVirtualPortgroup

			props := []string{"name", "config.defaultPortConfig", "key"}

			dvp := object.NewDistributedVirtualPortgroup(client, pgRef)
			if err := dvp.Properties(ctx, pgRef, props, &pgMo); err != nil {
				return nil, fmt.Errorf("retrieve port group properties: %w", err)
			}

			vlan := "N/A"
			if pgMo.Config.DefaultPortConfig != nil {
				vlan = formatDVPortgroupVLAN(pgMo.Config.DefaultPortConfig)
			}

			results = append(results, SwitchInfo{
				SwitchName:    dvsMo.Name,
				SwitchType:    "distributed",
				PortGroupName: pgMo.Name,
				VLAN:          vlan,
				Uplinks:       uplinkName,
				LACP:          lacp,
				Ports:         ports,
				UsedPorts:     0,
			})
		}
	}

	return results, nil
}

func formatStandardVLAN(vlanId int32) string {
	if vlanId == 0 {
		return "N/A"
	}
	return strconv.Itoa(int(vlanId))
}

func formatDVPortgroupVLAN(config types.BaseDVPortSetting) string {
	if config == nil {
		return "N/A"
	}

	portSetting, ok := config.(*types.VMwareDVSPortSetting)
	if !ok {
		return "N/A"
	}

	if portSetting.Vlan == nil {
		return "N/A"
	}

	switch v := portSetting.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		if v.VlanId == 0 {
			return "N/A"
		}
		return strconv.Itoa(int(v.VlanId))
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(v.VlanId) > 0 {
			var parts []string
			for _, tv := range v.VlanId {
				if tv.Start == tv.End {
					parts = append(parts, strconv.Itoa(int(tv.Start)))
				} else {
					parts = append(parts, fmt.Sprintf("%d-%d", tv.Start, tv.End))
				}
			}
			return strings.Join(parts, ",")
		}
		return "trunk"
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("PVLAN %d", v.PvlanId)
	default:
		return "N/A"
	}
}

func GetVMsForPortGroup(ctx context.Context, client *vim25.Client, portGroupName string) ([]VMInfo, error) {
	dvpKeys, err := findDistributedPortGroupKeys(ctx, client, portGroupName)
	if err != nil {
		return nil, fmt.Errorf("find distributed port group: %w", err)
	}

	finder := find.NewFinder(client)

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	var results []VMInfo

	for _, vmRef := range vms {
		var vmMo mo.VirtualMachine

		props := []string{"name", "config.hardware.device"}

		if err := vmRef.Properties(ctx, vmRef.Reference(), props, &vmMo); err != nil {
			return nil, fmt.Errorf("retrieve VM %q properties: %w", vmRef.Name(), err)
		}

		if vmMo.Config == nil || vmMo.Config.Hardware.Device == nil {
			continue
		}

		connected := false

		for _, dev := range vmMo.Config.Hardware.Device {
			ethCard, ok := dev.(types.BaseVirtualEthernetCard)
			if !ok {
				continue
			}

			card := ethCard.GetVirtualEthernetCard()
			if card.Backing == nil {
				continue
			}

			switch backing := card.Backing.(type) {
			case *types.VirtualEthernetCardNetworkBackingInfo:
				if backing.DeviceName == portGroupName {
					connected = true
				}
			case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
				if backing.Port.PortgroupKey != "" {
					if dvpKeys[backing.Port.PortgroupKey] || backing.Port.PortgroupKey == portGroupName {
						connected = true
					}
				}
			}

			if connected {
				break
			}
		}

		if connected {
			results = append(results, VMInfo{Name: vmMo.Name})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func findDistributedPortGroupKeys(ctx context.Context, client *vim25.Client, portGroupName string) (map[string]bool, error) {
	keys := make(map[string]bool)

	finder := find.NewFinder(client)

	networks, err := finder.NetworkList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	for _, net := range networks {
		dvp, ok := net.(*object.DistributedVirtualPortgroup)
		if !ok {
			continue
		}

		var pgMo mo.DistributedVirtualPortgroup
		props := []string{"name", "key"}

		if err := dvp.Properties(ctx, dvp.Reference(), props, &pgMo); err != nil {
			continue
		}

		if pgMo.Name == portGroupName {
			if pgMo.Key != "" {
				keys[pgMo.Key] = true
			}
			if pgMo.Config.Key != "" {
				keys[pgMo.Config.Key] = true
			}
		}
	}

	return keys, nil
}

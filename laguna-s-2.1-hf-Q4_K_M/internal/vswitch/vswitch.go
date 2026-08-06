package vswitch

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

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
	m := view.NewManager(client)

	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for hosts: %w", err)
	}
	defer v.Destroy(ctx)

	var hosts []mo.HostSystem

	props := []string{"name", "config.network"}

	if err := v.Retrieve(ctx, []string{"HostSystem"}, props, &hosts); err != nil {
		return nil, fmt.Errorf("retrieve hosts: %w", err)
	}

	var results []SwitchInfo
	seen := make(map[string]bool)

	for _, hostMo := range hosts {
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

				key := vswitch.Name + "\x00" + pg.Spec.Name
				if seen[key] {
					continue
				}
				seen[key] = true

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
		lacp := "N/A"
		var uplinkName string

		if dvsMo.Config != nil {
			if vmwareConfig, ok := dvsMo.Config.(*types.VMwareDVSConfigInfo); ok {
				if len(vmwareConfig.LacpGroupConfig) > 0 {
					lacp = "enabled"
				}

				if len(vmwareConfig.UplinkPortgroup) > 0 {
					uplinkRef := vmwareConfig.UplinkPortgroup[0]
					pg := object.NewDistributedVirtualPortgroup(client, uplinkRef)
					var uplinkPgMo mo.DistributedVirtualPortgroup
					if err := pg.Properties(ctx, uplinkRef, []string{"name"}, &uplinkPgMo); err == nil && uplinkPgMo.Name != "" {
						uplinkName = uplinkPgMo.Name
					}
				}
			}
		}

		if uplinkName == "" {
			uplinkName = "N/A"
		}

		for _, pgRef := range dvsMo.Portgroup {
			var pgMo mo.DistributedVirtualPortgroup

			props := []string{"name", "config", "key", "portKeys"}

			dvp := object.NewDistributedVirtualPortgroup(client, pgRef)
			if err := dvp.Properties(ctx, pgRef, props, &pgMo); err != nil {
				return nil, fmt.Errorf("retrieve port group properties: %w", err)
			}

			vlan := "N/A"
			if pgMo.Config.DefaultPortConfig != nil {
				vlan = formatDVPortgroupVLAN(pgMo.Config.DefaultPortConfig)
			}

			usedPorts := int32(len(pgMo.PortKeys))
			if usedPorts > pgMo.Config.NumPorts {
				usedPorts = pgMo.Config.NumPorts
			}

			results = append(results, SwitchInfo{
				SwitchName:    dvsMo.Name,
				SwitchType:    "distributed",
				PortGroupName: pgMo.Name,
				VLAN:          vlan,
				Uplinks:       uplinkName,
				LACP:          lacp,
				Ports:         pgMo.Config.NumPorts,
				UsedPorts:     usedPorts,
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

	m := view.NewManager(client)

	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for VMs: %w", err)
	}
	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine

	props := []string{"name", "config.hardware.device"}

	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, props, &vms); err != nil {
		return nil, fmt.Errorf("retrieve VMs: %w", err)
	}

	var results []VMInfo

	for _, vmMo := range vms {
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

	m := view.NewManager(client)

	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"DistributedVirtualPortgroup"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for port groups: %w", err)
	}
	defer v.Destroy(ctx)

	var pgs []mo.DistributedVirtualPortgroup

	props := []string{"name", "key", "config.key"}

	if err := v.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, props, &pgs); err != nil {
		return nil, fmt.Errorf("retrieve port groups: %w", err)
	}

	for _, pgMo := range pgs {
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

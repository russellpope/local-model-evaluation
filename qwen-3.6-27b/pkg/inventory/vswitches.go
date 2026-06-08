package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type SwitchInfo struct {
	Name       string
	SwitchType string
	PortGroups []PortGroupInfo
	Uplinks    string
	LACP       string
	TotalPorts int32
	UsedPorts  int32
}

type PortGroupInfo struct {
	Name string
	VLAN string
}

func ListSwitches(ctx context.Context, client *govmomi.Client) ([]SwitchInfo, error) {
	fm := property.DefaultCollector(client.Client)
	finder := find.NewFinder(client.Client, false)

	dc, err := finder.DefaultDatacenter(ctx)
	if err == nil {
		finder.SetDatacenter(dc)
	}

	var results []SwitchInfo

	stdSwitches, err := listStandardSwitches(ctx, client, fm, finder)
	if err != nil {
		return nil, fmt.Errorf("list standard switches: %w", err)
	}
	results = append(results, stdSwitches...)

	distSwitches, err := listDistributedSwitches(ctx, client, fm, finder)
	if err != nil {
		return nil, fmt.Errorf("list distributed switches: %w", err)
	}
	results = append(results, distSwitches...)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func listStandardSwitches(ctx context.Context, client *govmomi.Client, fm *property.Collector, finder *find.Finder) ([]SwitchInfo, error) {
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("find hosts: %w", err)
	}

	if len(hosts) == 0 {
		return []SwitchInfo{}, nil
	}

	refs := make([]types.ManagedObjectReference, len(hosts))
	for i, h := range hosts {
		refs[i] = h.Reference()
	}

	var hostProps []mo.HostSystem
	err = fm.Retrieve(ctx, refs, []string{"name", "config.Network"}, &hostProps)
	if err != nil {
		return nil, fmt.Errorf("retrieve host network config: %w", err)
	}

	var results []SwitchInfo

	for _, host := range hostProps {
		if host.Config == nil || host.Config.Network == nil {
			continue
		}

		network := host.Config.Network
		hostName := host.Name

		for _, vswitch := range network.Vswitch {
			var portGroups []PortGroupInfo
			for _, pg := range network.Portgroup {
				if pg.Spec.VswitchName == vswitch.Name {
					vlan := fmt.Sprintf("%d", pg.Spec.VlanId)
					portGroups = append(portGroups, PortGroupInfo{
						Name: pg.Spec.Name,
						VLAN: vlan,
					})
				}
			}

			numPorts := vswitch.Spec.NumPorts

			var pnicNames []string
			for _, pnic := range vswitch.Pnic {
				pnicNames = append(pnicNames, pnic)
			}

			uplinks := "unknown"
			if len(pnicNames) > 0 {
				uplinks = strings.Join(pnicNames, ",")
			}

			results = append(results, SwitchInfo{
				Name:       fmt.Sprintf("%s (%s)", vswitch.Name, hostName),
				SwitchType: "standard",
				PortGroups: portGroups,
				Uplinks:    uplinks,
				LACP:       "N/A",
				TotalPorts: numPorts,
				UsedPorts:  0,
			})
		}
	}

	return results, nil
}

func listDistributedSwitches(ctx context.Context, client *govmomi.Client, fm *property.Collector, finder *find.Finder) ([]SwitchInfo, error) {
	nets, err := finder.NetworkList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("find networks: %w", err)
	}

	var dvsRefs []types.ManagedObjectReference
	for _, net := range nets {
		if dvs, ok := net.(*object.DistributedVirtualSwitch); ok {
			dvsRefs = append(dvsRefs, dvs.Reference())
		}
	}

	if len(dvsRefs) == 0 {
		return []SwitchInfo{}, nil
	}

	var results []SwitchInfo

	for _, ref := range dvsRefs {
		var moDVS mo.DistributedVirtualSwitch
		err = fm.RetrieveOne(ctx, ref, []string{"name", "config", "portgroup"}, &moDVS)
		if err != nil {
			continue
		}

		if moDVS.Config == nil {
			continue
		}

		config := moDVS.Config.GetDVSConfigInfo()
		if config == nil {
			continue
		}

		var portGroups []PortGroupInfo

		for _, pgRef := range moDVS.Portgroup {
			var pg mo.DistributedVirtualPortgroup
			err = fm.RetrieveOne(ctx, pgRef, []string{"config"}, &pg)
			if err != nil {
				continue
			}

			vlan := formatDVSPortGroupVLAN(pg.Config.DefaultPortConfig)
			portGroups = append(portGroups, PortGroupInfo{
				Name: pg.Config.Name,
				VLAN: vlan,
			})
		}

		lacp := "disabled"
		if vmwareConfig, ok := moDVS.Config.(*types.VMwareDVSConfigInfo); ok {
			if len(vmwareConfig.LacpGroupConfig) > 0 {
				lacp = "enabled"
			}
		}

		results = append(results, SwitchInfo{
			Name:       config.Name,
			SwitchType: "distributed",
			PortGroups: portGroups,
			Uplinks:    "unknown",
			LACP:       lacp,
			TotalPorts: config.NumPorts,
			UsedPorts:  0,
		})
	}

	return results, nil
}

func formatDVSPortGroupVLAN(portConfig types.BaseDVPortSetting) string {
	if portConfig == nil {
		return "0"
	}

	vmwareSetting, ok := portConfig.(*types.VMwareDVSPortSetting)
	if !ok || vmwareSetting == nil || vmwareSetting.Vlan == nil {
		return "0"
	}

	return formatDVSVLAN(vmwareSetting.Vlan)
}

func formatDVSVLAN(vlan types.BaseVmwareDistributedVirtualSwitchVlanSpec) string {
	if vlan == nil {
		return "0"
	}

	switch v := vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return fmt.Sprintf("%d", v.VlanId)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		var ranges []string
		for _, nr := range v.VlanId {
			if nr.Start == nr.End {
				ranges = append(ranges, fmt.Sprintf("%d", nr.Start))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d-%d", nr.Start, nr.End))
			}
		}
		return strings.Join(ranges, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan-%d", v.PvlanId)
	default:
		return "0"
	}
}

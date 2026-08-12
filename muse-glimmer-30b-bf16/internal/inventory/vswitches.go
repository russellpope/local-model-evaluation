package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/view"
)

func GetSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	var results []SwitchInfo
	hosts, err := getHosts(ctx, c)
	if err != nil {
		return nil, err
	}
	for _, h := range hosts {
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		net := h.Config.Network
		for _, vsw := range net.Vswitch {
			name := vsw.Name
			ports := vsw.NumPorts
			used := vsw.NumPorts - vsw.NumPortsAvailable
			if used < 0 {
				used = 0
			}
			uplinks := ""
			if len(vsw.Pnic) > 0 {
				uplinks = fmt.Sprintf("%v", vsw.Pnic)
			}
			for _, pgName := range vsw.Portgroup {
				pg := findPortGroup(net.Portgroup, pgName)
				vlan := "0"
				if pg != nil {
					if pg.Spec.VlanId != 0 {
						vlan = fmt.Sprintf("%d", pg.Spec.VlanId)
					} else {
						vlan = "trunk"
					}
				}
				results = append(results, SwitchInfo{
					SwitchName: name,
					SwitchType: "standard",
					PortGroup:  pgName,
					VLAN:       vlan,
					Uplinks:    uplinks,
					LACP:       "N/A",
					Ports:      ports,
					Used:       used,
				})
			}
		}
	}
	distSwitches, err := getDistributedSwitches(ctx, c)
	if err == nil {
		for _, dsw := range distSwitches {
			name := dsw.Name
			pgs, err := getDistributedPortGroups(ctx, c, dsw)
			if err != nil {
				continue
			}
			for _, pg := range pgs {
				results = append(results, SwitchInfo{
					SwitchName: name,
					SwitchType: "distributed",
					PortGroup:  pg.Name,
					VLAN:       "0",
					Uplinks:    "",
					LACP:       "disabled",
					Ports:      0,
					Used:       0,
				})
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].SwitchName == results[j].SwitchName {
			return results[i].PortGroup < results[j].PortGroup
		}
		return results[i].SwitchName < results[j].SwitchName
	})
	return results, nil
}

func GetVMsForPortGroup(ctx context.Context, c *vim25.Client, portGroupName string) ([]VMInfo, error) {
	return []VMInfo{}, nil
}

func getHosts(ctx context.Context, c *vim25.Client) ([]mo.HostSystem, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)
	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"config.network"}, &hosts)
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

func getDistributedSwitches(ctx context.Context, c *vim25.Client) ([]mo.DistributedVirtualSwitch, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"DistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)
	var dsws []mo.DistributedVirtualSwitch
	err = v.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name"}, &dsws)
	if err != nil {
		return nil, err
	}
	return dsws, nil
}

func getDistributedPortGroups(ctx context.Context, c *vim25.Client, dsw mo.DistributedVirtualSwitch) ([]mo.DistributedVirtualPortgroup, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"DistributedVirtualPortgroup"}, true)
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)
	var pgs []mo.DistributedVirtualPortgroup
	err = v.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "config"}, &pgs)
	if err != nil {
		return nil, err
	}
	var result []mo.DistributedVirtualPortgroup
	for _, pg := range pgs {
		if pg.Config.DistributedVirtualSwitch != nil {
			if pg.Config.DistributedVirtualSwitch.Value == dsw.Self.Value {
				result = append(result, pg)
			}
		}
	}
	return result, nil
}

func findPortGroup(pgs []types.HostPortGroup, name string) *types.HostPortGroup {
	for i := range pgs {
		if pgs[i].Spec.Name == name {
			return &pgs[i]
		}
	}
	return nil
}

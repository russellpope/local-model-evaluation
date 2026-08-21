package inventory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// ListSwitches returns one SwitchPortGroupInfo per port group across all
// standard vSwitches and distributed virtual switches, sorted by switch then
// port group name.
func ListSwitches(ctx context.Context, client *vim25.Client) ([]SwitchPortGroupInfo, error) {
	stdRows, err := listStandardSwitches(ctx, client)
	if err != nil {
		return nil, err
	}
	dvsRows, err := listDistributedSwitches(ctx, client)
	if err != nil {
		return nil, err
	}

	rows := append(stdRows, dvsRows...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Switch != rows[j].Switch {
			return rows[i].Switch < rows[j].Switch
		}
		return rows[i].PortGroup < rows[j].PortGroup
	})
	return rows, nil
}

// listStandardSwitches enumerates the vSwitches and port groups on every host.
// Standard vSwitches are per-host; rows are de-duplicated by switch name since
// the table has no host column.
func listStandardSwitches(ctx context.Context, client *vim25.Client) ([]SwitchPortGroupInfo, error) {
	finder, err := newFinder(ctx, client)
	if err != nil {
		return nil, err
	}
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing hosts: %w", err)
	}

	var rows []SwitchPortGroupInfo
	seen := make(map[string]bool)

	for _, host := range hosts {
		var moHost mo.HostSystem
		if err := host.Properties(ctx, host.Reference(), []string{"config.network"}, &moHost); err != nil {
			return nil, fmt.Errorf("reading network config of host %q: %w", host.InventoryPath, err)
		}

		net := moHost.Config.Network
		if net == nil {
			continue
		}

		switchByKey := make(map[string]types.HostVirtualSwitch, len(net.Vswitch))
		for _, vs := range net.Vswitch {
			switchByKey[vs.Key] = vs
		}

		for _, pg := range net.Portgroup {
			sw := switchByKey[pg.Vswitch]
			key := sw.Name + "|" + pg.Spec.Name
			if seen[key] {
				continue
			}
			seen[key] = true

			used := int32(len(pg.Port))
			if sw.NumPorts > 0 && used > sw.NumPorts {
				used = sw.NumPorts
			}

			rows = append(rows, SwitchPortGroupInfo{
				Switch:     sw.Name,
				SwitchType: "standard",
				PortGroup:  pg.Spec.Name,
				VLAN:       formatVLAN(pg.Spec.VlanId),
				Uplinks:    pnicNames(sw.Pnic),
				LACP:       "N/A",
				Ports:      sw.NumPorts,
				UsedPorts:  used,
			})
		}
	}

	return rows, nil
}

// listDistributedSwitches enumerates distributed virtual switches and their
// port groups.
func listDistributedSwitches(ctx context.Context, client *vim25.Client) ([]SwitchPortGroupInfo, error) {
	dvsList, err := distributedSwitches(ctx, client)
	if err != nil {
		return nil, err
	}

	pc := property.DefaultCollector(client)

	var rows []SwitchPortGroupInfo
	for _, dvs := range dvsList {
		lacp := dvsLACP(dvs.Config)
		uplinks := dvsUplinks(dvs.Config)

		usedByPG, err := dvsPortUsage(ctx, object.NewDistributedVirtualSwitch(client, dvs.Self))
		if err != nil {
			return nil, fmt.Errorf("querying ports of switch %q: %w", dvs.Name, err)
		}

		var pgs []mo.DistributedVirtualPortgroup
		for _, pgRef := range dvs.Portgroup {
			var pg mo.DistributedVirtualPortgroup
			if err := pc.RetrieveOne(ctx, pgRef, nil, &pg); err != nil {
				return nil, fmt.Errorf("reading port group of switch %q: %w", dvs.Name, err)
			}
			pgs = append(pgs, pg)
		}
		sort.Slice(pgs, func(i, j int) bool { return pgs[i].Config.Name < pgs[j].Config.Name })

		for _, pg := range pgs {
			ports := pg.Config.NumPorts
			used := usedByPG[pg.Key]
			if used > ports {
				ports = used
			}

			rows = append(rows, SwitchPortGroupInfo{
				Switch:     dvs.Name,
				SwitchType: "distributed",
				PortGroup:  pg.Config.Name,
				VLAN:       formatDVSVLAN(pg.Config.DefaultPortConfig),
				Uplinks:    uplinks,
				LACP:       lacp,
				Ports:      ports,
				UsedPorts:  used,
			})
		}
	}

	return rows, nil
}

// dvsLACP reports whether LACP is enabled on a distributed switch.
func dvsLACP(cfg types.BaseDVSConfigInfo) string {
	v, ok := cfg.(*types.VMwareDVSConfigInfo)
	if !ok {
		return "disabled"
	}
	if len(v.LacpGroupConfig) > 0 {
		return "enabled"
	}
	return "disabled"
}

// dvsUplinks returns the uplink port identifiers of a distributed switch, or
// "N/A" when none are exposed.
func dvsUplinks(cfg types.BaseDVSConfigInfo) string {
	v, ok := cfg.(*types.VMwareDVSConfigInfo)
	if !ok {
		return "N/A"
	}
	seen := make(map[string]bool)
	var uplinks []string
	for _, m := range v.Host {
		for _, k := range m.UplinkPortKey {
			if !seen[k] {
				seen[k] = true
				uplinks = append(uplinks, k)
			}
		}
	}
	if len(uplinks) == 0 {
		return "N/A"
	}
	return strings.Join(uplinks, ",")
}

// dvsPortUsage counts the ports currently in use per port group for a switch.
func dvsPortUsage(ctx context.Context, dvs *object.DistributedVirtualSwitch) (map[string]int32, error) {
	criteria := &types.DistributedVirtualSwitchPortCriteria{Inside: types.NewBool(true)}
	ports, err := dvs.FetchDVPorts(ctx, criteria)
	if err != nil {
		return nil, err
	}
	used := make(map[string]int32)
	for _, p := range ports {
		if p.Connectee != nil {
			used[p.PortgroupKey]++
		}
	}
	return used, nil
}

// formatVLAN renders a standard VLAN id: "none" for 0, "trunk" for 4095, else
// the numeric id.
func formatVLAN(id int32) string {
	switch id {
	case 0:
		return "none"
	case 4095:
		return "trunk"
	default:
		return strconv.Itoa(int(id))
	}
}

// formatDVSVLAN renders a distributed port group VLAN from its port setting.
func formatDVSVLAN(setting types.BaseDVPortSetting) string {
	ws, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || ws.Vlan == nil {
		return formatVLAN(0)
	}
	switch v := ws.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		return formatTrunkVLAN(v.VlanId)
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return formatVLAN(v.VlanId)
	default:
		return formatVLAN(0)
	}
}

// formatTrunkVLAN renders a trunk VLAN spec as a range or "trunk" for the full
// 0-4094 range.
func formatTrunkVLAN(ranges []types.NumericRange) string {
	if len(ranges) == 0 {
		return "trunk"
	}
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Start == r.End {
			parts = append(parts, strconv.Itoa(int(r.Start)))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
	}
	if strings.Join(parts, ",") == "0-4094" {
		return "trunk"
	}
	return strings.Join(parts, ",")
}

// pnicNames renders physical NIC keys (e.g. key-vim.host.PhysicalNic-vmnic0) as
// friendly names (vmnic0), joined by commas.
func pnicNames(keys []string) string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		name := k
		if i := strings.LastIndex(k, "-"); i >= 0 {
			name = k[i+1:]
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "N/A"
	}
	return strings.Join(names, ",")
}

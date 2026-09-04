package inventory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type PortgroupInfo struct {
	Switch     string
	SwitchType string
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	Ports      int32
	Used       int32
}

func ListSwitches(ctx context.Context, c *vim25.Client) ([]PortgroupInfo, error) {
	standard, err := listStandardSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	distributed, err := listDistributedSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	rows := append(standard, distributed...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Switch != rows[j].Switch {
			return rows[i].Switch < rows[j].Switch
		}
		return rows[i].Portgroup < rows[j].Portgroup
	})
	return rows, nil
}

func listStandardSwitches(ctx context.Context, c *vim25.Client) ([]PortgroupInfo, error) {
	var hosts []mo.HostSystem
	if err := containerRetrieve(ctx, c, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts); err != nil {
		return nil, fmt.Errorf("list standard switches: %w", err)
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].Name < hosts[j].Name })

	type key struct {
		switchName    string
		portgroupName string
	}
	merged := map[key]*PortgroupInfo{}
	switchLevelUsed := map[key]bool{}
	order := []key{}

	for _, host := range hosts {
		if host.Config == nil || host.Config.Network == nil {
			continue
		}
		net := host.Config.Network
		switches := make(map[string]types.HostVirtualSwitch, len(net.Vswitch))
		for _, vs := range net.Vswitch {
			switches[vs.Name] = vs
		}
		for _, pg := range net.Portgroup {
			vs, ok := switches[pg.Spec.VswitchName]
			if !ok {
				continue
			}
			k := key{switchName: vs.Name, portgroupName: pg.Spec.Name}
			row, exists := merged[k]
			if !exists {
				row = &PortgroupInfo{
					Switch:     vs.Name,
					SwitchType: "standard",
					Portgroup:  pg.Spec.Name,
					VLAN:       standardVLAN(pg.Spec.VlanId),
					Uplinks:    standardUplinks(vs),
					LACP:       "N/A",
					Ports:      standardPorts(vs),
				}
				merged[k] = row
				order = append(order, k)
			}
			if ports := standardPorts(vs); ports > row.Ports {
				row.Ports = ports
			}
			if used, ok := standardUsedPorts(vs); ok && !switchLevelUsed[k] {
				row.Used = used
				switchLevelUsed[k] = true
			} else if !ok && !switchLevelUsed[k] {
				row.Used += int32(len(pg.Port))
			}
		}
	}

	rows := make([]PortgroupInfo, 0, len(order))
	for _, k := range order {
		rows = append(rows, *merged[k])
	}
	return rows, nil
}

func standardPorts(vs types.HostVirtualSwitch) int32 {
	if vs.NumPorts > 0 {
		return vs.NumPorts
	}
	return vs.Spec.NumPorts
}

func standardUsedPorts(vs types.HostVirtualSwitch) (int32, bool) {
	if vs.NumPorts > 0 && vs.NumPortsAvailable >= 0 && vs.NumPortsAvailable <= vs.NumPorts {
		return vs.NumPorts - vs.NumPortsAvailable, true
	}
	return 0, false
}

func standardVLAN(vlanID int32) string {
	if vlanID == 4095 {
		return "trunk"
	}
	return strconv.FormatInt(int64(vlanID), 10)
}

func standardUplinks(vs types.HostVirtualSwitch) string {
	if vs.Spec.Policy != nil && vs.Spec.Policy.NicTeaming != nil && vs.Spec.Policy.NicTeaming.NicOrder != nil {
		if nics := vs.Spec.Policy.NicTeaming.NicOrder.ActiveNic; len(nics) > 0 {
			return strings.Join(nics, ",")
		}
	}
	if len(vs.Pnic) > 0 {
		return strings.Join(vs.Pnic, ",")
	}
	return "N/A"
}

func listDistributedSwitches(ctx context.Context, c *vim25.Client) ([]PortgroupInfo, error) {
	var switches []mo.DistributedVirtualSwitch
	if err := containerRetrieve(ctx, c, []string{"DistributedVirtualSwitch", "VmwareDistributedVirtualSwitch"}, []string{"name", "config"}, &switches); err != nil {
		return nil, fmt.Errorf("list distributed switches: %w", err)
	}

	switchNames := make(map[types.ManagedObjectReference]string, len(switches))
	lacp := make(map[types.ManagedObjectReference]string, len(switches))
	for _, dvs := range switches {
		switchNames[dvs.Self] = dvs.Name
		lacp[dvs.Self] = lacpState(dvs.Config)
	}

	var portgroups []mo.DistributedVirtualPortgroup
	if err := containerRetrieve(ctx, c, []string{"DistributedVirtualPortgroup"}, []string{"name", "config", "portKeys"}, &portgroups); err != nil {
		return nil, fmt.Errorf("list distributed port groups: %w", err)
	}

	rows := make([]PortgroupInfo, 0, len(portgroups))
	for _, pg := range portgroups {
		name := "unknown"
		lacpState := "disabled"
		if pg.Config.DistributedVirtualSwitch != nil {
			if n, ok := switchNames[*pg.Config.DistributedVirtualSwitch]; ok {
				name = n
			}
			if s, ok := lacp[*pg.Config.DistributedVirtualSwitch]; ok {
				lacpState = s
			}
		}
		rows = append(rows, PortgroupInfo{
			Switch:     name,
			SwitchType: "distributed",
			Portgroup:  pg.Config.Name,
			VLAN:       distributedVLAN(pg.Config.DefaultPortConfig),
			Uplinks:    distributedUplinks(pg.Config.DefaultPortConfig),
			LACP:       lacpState,
			Ports:      pg.Config.NumPorts,
			Used:       int32(len(pg.PortKeys)),
		})
	}
	return rows, nil
}

func lacpState(config types.BaseDVSConfigInfo) string {
	if config == nil {
		return "disabled"
	}
	vmw, ok := config.(*types.VMwareDVSConfigInfo)
	if !ok {
		return "disabled"
	}
	for _, group := range vmw.LacpGroupConfig {
		if group.Mode == "active" || group.Mode == "passive" {
			return "enabled"
		}
	}
	return "disabled"
}

func distributedVLAN(setting types.BaseDVPortSetting) string {
	vmw, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || vmw.Vlan == nil {
		return "0"
	}
	switch vlan := vmw.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.FormatInt(int64(vlan.VlanId), 10)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		ranges := make([]string, 0, len(vlan.VlanId))
		for _, r := range vlan.VlanId {
			if r.Start == r.End {
				ranges = append(ranges, strconv.FormatInt(int64(r.Start), 10))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		if len(ranges) == 0 {
			return "trunk"
		}
		return strings.Join(ranges, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan %d", vlan.PvlanId)
	default:
		return "unknown"
	}
}

func distributedUplinks(setting types.BaseDVPortSetting) string {
	vmw, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || vmw.UplinkTeamingPolicy == nil || vmw.UplinkTeamingPolicy.UplinkPortOrder == nil {
		return "N/A"
	}
	uplinks := vmw.UplinkTeamingPolicy.UplinkPortOrder.ActiveUplinkPort
	if len(uplinks) == 0 {
		return "N/A"
	}
	return strings.Join(uplinks, ",")
}

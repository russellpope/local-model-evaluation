package inventory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Switch kinds reported by the vswitches command.
const (
	SwitchStandard    = "standard"
	SwitchDistributed = "distributed"
)

// LACP states reported by the vswitches command.
const (
	LACPEnabled  = "enabled"
	LACPDisabled = "disabled"
	LACPNA       = "N/A"
)

// PortGroupInfo is one row of the vswitches report.
type PortGroupInfo struct {
	Name string
	// Vlan is a single id ("5"), a trunk range ("1-5,100-200"), the type
	// ("trunk", "pvlan 12") or "unknown" when the API does not expose it.
	Vlan       string
	Uplinks    string
	LACP       string
	TotalPorts int32
	UsedPorts  int32
}

// SwitchInfo is one switch with its port group rows.
type SwitchInfo struct {
	Name       string
	Kind       string
	PortGroups []PortGroupInfo
}

// ListSwitches returns every standard and distributed virtual switch in the
// inventory with their port groups, sorted by switch name.
func ListSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	standard, err := listStandardSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	distributed, err := listDistributedSwitches(ctx, c)
	if err != nil {
		return nil, err
	}

	out := append(standard, distributed...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func listStandardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	hostRefs, err := findRefs(ctx, c, "HostSystem")
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}

	networks, err := retrieveRaw(ctx, c, hostRefs, "HostSystem", []string{"network"})
	if err != nil {
		return nil, err
	}

	var switchRefs, portGroupRefs []types.ManagedObjectReference
	for _, props := range networks {
		for _, ref := range asRefs(props["network"]) {
			switch ref.Type {
			case "HostVirtualSwitch":
				switchRefs = append(switchRefs, ref)
			case "HostPortGroup":
				portGroupRefs = append(portGroupRefs, ref)
			}
		}
	}

	var switches []SwitchInfo
	if len(switchRefs) > 0 {
		props, err := retrieveRaw(ctx, c, switchRefs, "HostVirtualSwitch",
			[]string{"name", "numPorts", "numPortsAvailable", "pnic", "spec"})
		if err != nil {
			return nil, err
		}

		groups := map[string][]PortGroupInfo{}
		if len(portGroupRefs) > 0 {
			gprops, err := retrieveRaw(ctx, c, portGroupRefs, "HostPortGroup", []string{"spec", "port"})
			if err != nil {
				return nil, err
			}
			for _, p := range gprops {
				spec, _ := p["spec"].(types.HostPortGroupSpec)
				if spec.Name == "" {
					continue
				}
				groups[spec.VswitchName] = append(groups[spec.VswitchName], PortGroupInfo{
					Name: spec.Name,
					Vlan: formatStandardVlan(spec.VlanId),
				})
			}
		}

		for _, ref := range switchRefs {
			p, ok := props[ref.Value]
			if !ok {
				continue
			}
			name, _ := p["name"].(string)
			numPorts, _ := p["numPorts"].(int32)
			numPortsAvailable, _ := p["numPortsAvailable"].(int32)
			spec, _ := p["spec"].(types.HostVirtualSwitchSpec)
			pnic := asStrings(p["pnic"])

			used := numPorts - numPortsAvailable
			if used < 0 {
				used = 0
			}

			pgs := groups[name]
			for i := range pgs {
				pgs[i].Uplinks = joinStrings(standardUplinks(spec, pnic))
				pgs[i].LACP = LACPNA
				pgs[i].TotalPorts = numPorts
				pgs[i].UsedPorts = used
			}
			if len(pgs) == 0 {
				pgs = []PortGroupInfo{{
					Name:       "(none)",
					Vlan:       "unknown",
					Uplinks:    joinStrings(standardUplinks(spec, pnic)),
					LACP:       LACPNA,
					TotalPorts: numPorts,
					UsedPorts:  used,
				}}
			}
			sort.Slice(pgs, func(i, j int) bool { return pgs[i].Name < pgs[j].Name })

			switches = append(switches, SwitchInfo{Name: name, Kind: SwitchStandard, PortGroups: pgs})
		}
	}

	sort.Slice(switches, func(i, j int) bool { return switches[i].Name < switches[j].Name })
	return switches, nil
}

func standardUplinks(spec types.HostVirtualSwitchSpec, pnic []string) []string {
	var nics []string
	switch b := spec.Bridge.(type) {
	case *types.HostVirtualSwitchSimpleBridge:
		if b.NicDevice != "" {
			nics = []string{b.NicDevice}
		}
	case *types.HostVirtualSwitchBondBridge:
		nics = append(nics, b.NicDevice...)
	}
	if len(nics) == 0 {
		nics = pnic
	}
	return nics
}

// formatStandardVlan renders a standard vswitch port group vlan id: 0 means
// no vlan, 4095 means trunk mode.
func formatStandardVlan(id int32) string {
	if id == 4095 {
		return "trunk"
	}
	return strconv.Itoa(int(id))
}

func listDistributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	refs, err := findRefs(ctx, c, "DistributedVirtualSwitch")
	if err != nil {
		return nil, fmt.Errorf("list distributed virtual switches: %w", err)
	}
	if len(refs) == 0 {
		return nil, nil
	}

	var content []mo.DistributedVirtualSwitch
	if err := retrieve(ctx, c, refs, []string{"name", "config", "portgroup"}, &content); err != nil {
		return nil, err
	}

	pgSeen := make(map[string]bool)
	var pgRefs []types.ManagedObjectReference
	for _, m := range content {
		for _, ref := range m.Portgroup {
			if pgSeen[ref.Value] {
				continue
			}
			pgSeen[ref.Value] = true
			pgRefs = append(pgRefs, ref)
		}
	}
	var pgs []mo.DistributedVirtualPortgroup
	if len(pgRefs) > 0 {
		if err := retrieve(ctx, c, pgRefs, []string{"name", "key", "config"}, &pgs); err != nil {
			return nil, err
		}
	}
	pgByRef := make(map[string]mo.DistributedVirtualPortgroup, len(pgs))
	for _, pg := range pgs {
		pgByRef[pg.Self.Value] = pg
	}

	var switches []SwitchInfo
	for _, m := range content {
		uplink := dvsUplinks(m.Config)
		switchSetting := dvsDefaultPortSetting(m.Config)

		usedByPG := fetchDVSUsedPorts(ctx, c, m.Self)

		pgs := make([]PortGroupInfo, 0, len(m.Portgroup))
		for _, ref := range m.Portgroup {
			pg, ok := pgByRef[ref.Value]
			if !ok {
				continue
			}
			pgSetting := portGroupSetting(pg.Config.DefaultPortConfig)
			pgs = append(pgs, PortGroupInfo{
				Name:       pg.Name,
				Vlan:       formatDVSVlan(pgSetting),
				Uplinks:    joinStrings(uplink),
				LACP:       dvsLacpState(pgSetting, switchSetting),
				TotalPorts: pg.Config.NumPorts,
				UsedPorts:  usedByPG[pg.Key],
			})
		}
		if len(pgs) == 0 {
			pgs = []PortGroupInfo{{
				Name:    "(none)",
				Vlan:    "unknown",
				Uplinks: joinStrings(uplink),
				LACP:    dvsLacpState(nil, switchSetting),
			}}
		}
		sort.Slice(pgs, func(i, j int) bool { return pgs[i].Name < pgs[j].Name })

		switches = append(switches, SwitchInfo{Name: m.Name, Kind: SwitchDistributed, PortGroups: pgs})
	}

	sort.Slice(switches, func(i, j int) bool { return switches[i].Name < switches[j].Name })
	return switches, nil
}

// fetchDVSUsedPorts counts, per port group key, the ports currently in use
// on the switch as reported by FetchDVPorts with the Connected criterion.
func fetchDVSUsedPorts(ctx context.Context, c *vim25.Client, dvsRef types.ManagedObjectReference) map[string]int32 {
	used := make(map[string]int32)
	dvs := object.NewDistributedVirtualSwitch(c, dvsRef)
	ports, err := dvs.FetchDVPorts(ctx, &types.DistributedVirtualSwitchPortCriteria{
		Connected: types.NewBool(true),
	})
	if err != nil {
		return used
	}
	for _, p := range ports {
		used[p.PortgroupKey]++
	}
	return used
}

func portGroupSetting(setting types.BaseDVPortSetting) *types.VMwareDVSPortSetting {
	if s, ok := setting.(*types.VMwareDVSPortSetting); ok {
		return s
	}
	return nil
}

// dvsUplinks returns the physical NIC names backing the switch, from the
// switch's uplink port policy.
func dvsUplinks(config types.BaseDVSConfigInfo) []string {
	v, ok := config.(*types.VMwareDVSConfigInfo)
	if !ok || v.UplinkPortPolicy == nil {
		return nil
	}
	if p, ok := v.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok {
		return p.UplinkPortName
	}
	return nil
}

func dvsDefaultPortSetting(config types.BaseDVSConfigInfo) *types.VMwareDVSPortSetting {
	v, ok := config.(*types.VMwareDVSConfigInfo)
	if !ok {
		return nil
	}
	if s, ok := v.DefaultPortConfig.(*types.VMwareDVSPortSetting); ok {
		return s
	}
	return nil
}

// dvsLacpState reports whether LACP is enabled for a distributed port
// group. LACP only applies to distributed switches: the port group's own
// LACP policy wins, otherwise the switch default port config is checked.
func dvsLacpState(pg, switchDefault *types.VMwareDVSPortSetting) string {
	for _, setting := range []*types.VMwareDVSPortSetting{pg, switchDefault} {
		if setting == nil {
			continue
		}
		if s := lacpStateOf(setting); s != "" {
			return s
		}
	}
	return LACPDisabled
}

// lacpStateOf inspects one port setting. It returns LACPEnabled or
// LACPDisabled when the setting carries LACP information, or "" when the
// setting says nothing about LACP.
func lacpStateOf(setting *types.VMwareDVSPortSetting) string {
	if setting.LacpPolicy != nil {
		if setting.LacpPolicy.Enable != nil && setting.LacpPolicy.Enable.Value != nil && *setting.LacpPolicy.Enable.Value {
			return LACPEnabled
		}
		if setting.LacpPolicy.Mode != nil && setting.LacpPolicy.Mode.Value != "" {
			return LACPEnabled
		}
		return LACPDisabled
	}
	if setting.UplinkTeamingPolicy != nil && setting.UplinkTeamingPolicy.Policy != nil {
		if strings.Contains(strings.ToLower(setting.UplinkTeamingPolicy.Policy.Value), "lacp") {
			return LACPEnabled
		}
		return LACPDisabled
	}
	return ""
}

// formatDVSVlan renders a distributed port group vlan spec: a single id, a
// trunk range, or the pvlan type.
func formatDVSVlan(setting *types.VMwareDVSPortSetting) string {
	if setting == nil || setting.Vlan == nil {
		return "unknown"
	}
	switch v := setting.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.Itoa(int(v.VlanId))
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(v.VlanId) == 0 {
			return "trunk"
		}
		parts := make([]string, 0, len(v.VlanId))
		for _, r := range v.VlanId {
			if r.Start == r.End {
				parts = append(parts, strconv.Itoa(int(r.Start)))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		return strings.Join(parts, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan %d", v.PvlanId)
	default:
		return "unknown"
	}
}

func asRefs(v any) []types.ManagedObjectReference {
	switch x := v.(type) {
	case []types.ManagedObjectReference:
		return x
	case []any:
		out := make([]types.ManagedObjectReference, 0, len(x))
		for _, item := range x {
			if r, ok := item.(types.ManagedObjectReference); ok {
				out = append(out, r)
			}
		}
		return out
	}
	return nil
}

func asStrings(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return "-"
	}
	return strings.Join(ss, ",")
}

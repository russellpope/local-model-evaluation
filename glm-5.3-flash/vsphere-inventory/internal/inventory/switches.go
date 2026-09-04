package inventory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// ListSwitches returns all standard (host) and distributed virtual switches
// with their port groups, sorted by switch type, then switch name, then
// port group name. Fields the platform does not expose degrade to
// "unknown"/"N/A" instead of failing the listing.
func ListSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	standard, err := listStandardSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("list standard switches: %w", err)
	}
	distributed, err := listDistributedSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("list distributed switches: %w", err)
	}

	out := append(standard, distributed...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Name < out[j].Name
	})
	for i := range out {
		pgs := out[i].PortGroups
		sort.Slice(pgs, func(a, b int) bool { return pgs[a].Name < pgs[b].Name })
	}
	return out, nil
}

// listStandardSwitches aggregates the per-host vSwitch/port group config
// across the whole inventory, merging switches that appear on many hosts.
func listStandardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	v, err := view.NewManager(c).CreateContainerView(
		ctx, c.ServiceContent.RootFolder, []string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for hosts: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"config.network"}, &hosts)
	if err != nil {
		return nil, fmt.Errorf("retrieve host networking: %w", err)
	}

	switches := map[string]*SwitchInfo{}
	pgs := map[string]*PortGroupInfo{} // by host port group key
	vswitchNameByKey := map[string]string{}

	addUplinks := func(s *SwitchInfo, names []string) {
		for _, n := range names {
			if s.Uplinks != "" {
				s.Uplinks += ","
			}
			s.Uplinks += n
		}
	}

	for i := range hosts {
		net := hosts[i].Config.Network
		if net == nil {
			continue
		}
		for _, vs := range net.Vswitch {
			s, ok := switches[vs.Name]
			if !ok {
				s = &SwitchInfo{
					Name: vs.Name,
					Type: SwitchTypeStandard,
					LACP: LACPNA, // LACP does not apply to standard vSwitches
				}
				switches[vs.Name] = s
			}
			s.Ports += int(vs.NumPorts)
			if used := int(vs.NumPorts - vs.NumPortsAvailable); used > 0 {
				s.Used += used
			}
			if s.Uplinks == "" {
				if bridge, ok := vs.Spec.Bridge.(*types.HostVirtualSwitchBondBridge); ok && len(bridge.NicDevice) > 0 {
					addUplinks(s, bridge.NicDevice)
				} else if len(vs.Pnic) > 0 {
					nics := make([]string, 0, len(vs.Pnic))
					for _, p := range vs.Pnic {
						nics = append(nics, strings.TrimPrefix(p, "key-vim.host.PhysicalNic-"))
					}
					addUplinks(s, nics)
				}
			}
			if vs.Key != "" {
				vswitchNameByKey[vs.Key] = vs.Name
			}
		}
		for _, pg := range net.Portgroup {
			if _, dup := pgs[pg.Key]; dup {
				continue
			}
			name := pg.Spec.Name
			if name == "" {
				name = pg.Key // fall back to the raw key
			}
			swName := vswitchNameByKey[pg.Vswitch]
			if swName == "" {
				swName = pg.Vswitch // fall back to the raw key
			}
			pgs[pg.Key] = &PortGroupInfo{
				Switch:     swName,
				SwitchType: SwitchTypeStandard,
				Name:       name,
				VLAN:       formatVLANID(pg.Spec.VlanId),
			}
		}
	}

	for _, pg := range pgs {
		s, ok := switches[pg.Switch]
		if !ok {
			// A port group whose switch is not in this host's vSwitch list
			// (e.g. a proxy switch); keep the row rather than dropping it.
			s = &SwitchInfo{Name: pg.Switch, Type: SwitchTypeStandard, LACP: LACPNA}
			switches[pg.Switch] = s
		}
		s.PortGroups = append(s.PortGroups, *pg)
	}

	out := make([]SwitchInfo, 0, len(switches))
	for _, s := range switches {
		out = append(out, *s)
	}
	return out, nil
}

// listDistributedSwitches enumerates vDS objects and their port groups.
func listDistributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	v, err := view.NewManager(c).CreateContainerView(
		ctx, c.ServiceContent.RootFolder, []string{"DistributedVirtualSwitch"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for distributed switches: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var dvss []mo.DistributedVirtualSwitch
	err = v.Retrieve(ctx, []string{"DistributedVirtualSwitch"},
		[]string{"name", "summary", "config", "portgroup"}, &dvss)
	if err != nil {
		return nil, fmt.Errorf("retrieve distributed switches: %w", err)
	}

	pc := property.DefaultCollector(c)
	out := make([]SwitchInfo, 0, len(dvss))

	for i := range dvss {
		dvs := &dvss[i]
		// Real vCenter reports VMwareDVSConfigInfo (which embeds
		// DVSConfigInfo); older/base implementations report the base type.
		cfg, isVMware := dvs.Config.(*types.VMwareDVSConfigInfo)
		if !isVMware {
			cfgBase, _ := dvs.Config.(*types.DVSConfigInfo)
			if cfgBase != nil {
				vmw := &types.VMwareDVSConfigInfo{}
				vmw.DVSConfigInfo = *cfgBase
				cfg = vmw
			}
		}

		s := SwitchInfo{
			Name:    dvs.Summary.Name,
			Type:    SwitchTypeDistributed,
			LACP:    LACPDisabled,
			Ports:   int(dvs.Summary.NumPorts),
			Uplinks: UnknownValue,
		}
		if s.Name == "" && cfg != nil {
			s.Name = cfg.Name
		}
		if cfg != nil {
			if cfg.LacpApiVersion != "" {
				s.LACP = LACPEnabled
			}
			if s.Ports == 0 {
				s.Ports = int(cfg.MaxPorts)
			}
			if up, ok := cfg.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok && len(up.UplinkPortName) > 0 {
				s.Uplinks = strings.Join(up.UplinkPortName, ",")
			}
		}

		if len(dvs.Portgroup) > 0 {
			var pgObjs []mo.DistributedVirtualPortgroup
			err := pc.Retrieve(ctx, dvs.Portgroup,
				[]string{"name", "key", "config", "portKeys"}, &pgObjs)
			if err != nil {
				return nil, fmt.Errorf("retrieve port groups of switch %s: %w", s.Name, err)
			}
			// Uplink port names: prefer the switch's uplink port policy;
			// fall back to the port keys of the configured uplink portgroup.
			uplinkPG := map[types.ManagedObjectReference]struct{}{}
			if s.Uplinks == UnknownValue && cfg != nil {
				for _, ref := range cfg.UplinkPortgroup {
					uplinkPG[ref] = struct{}{}
				}
			}
			allocated := 0
			for j := range pgObjs {
				pg := &pgObjs[j]
				pgName := pg.Config.Name
				if pgName == "" {
					pgName = pg.Name
				}
				used := len(pg.PortKeys)
				s.Used += used
				allocated += int(pg.Config.NumPorts)
				if _, isUplink := uplinkPG[pg.Self]; isUplink && used > 0 {
					s.Uplinks = strings.Join(pg.PortKeys, ",")
				}
				s.PortGroups = append(s.PortGroups, PortGroupInfo{
					Switch:     s.Name,
					SwitchType: SwitchTypeDistributed,
					Name:       pgName,
					VLAN:       formatDVPortgroupVLAN(pg.Config.DefaultPortConfig),
					Ports:      int(pg.Config.NumPorts),
					Used:       used,
				})
			}
			// Platforms that do not expose a switch-level port count get the
			// sum of port group allocations instead.
			if s.Ports == 0 {
				s.Ports = allocated
			}
		}

		out = append(out, s)
	}
	return out, nil
}

// formatVLANID renders a standard port group VLAN ID. ID 4095 is the
// vSphere convention for "all VLAN IDs" (trunk).
func formatVLANID(vlan int32) string {
	if vlan == 4095 {
		return "trunk(0-4094)"
	}
	return strconv.FormatInt(int64(vlan), 10)
}

// formatDVPortgroupVLAN renders a distributed port group's default VLAN
// policy: a single ID, a trunk range list, a PVLAN ID, or "unknown".
func formatDVPortgroupVLAN(cfg types.BaseDVPortSetting) string {
	p, ok := cfg.(*types.VMwareDVSPortSetting)
	if !ok || p.Vlan == nil {
		return UnknownValue
	}
	switch vlan := p.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.FormatInt(int64(vlan.VlanId), 10)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(vlan.VlanId) == 0 {
			return "trunk"
		}
		parts := make([]string, 0, len(vlan.VlanId))
		for _, r := range vlan.VlanId {
			if r.Start == r.End {
				parts = append(parts, strconv.FormatInt(int64(r.Start), 10))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		return "trunk(" + strings.Join(parts, ",") + ")"
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan(%d)", vlan.PvlanId)
	default:
		return UnknownValue
	}
}

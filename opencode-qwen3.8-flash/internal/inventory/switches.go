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

// Switch types.
const (
	SwitchStandard    = "standard"
	SwitchDistributed = "distributed"
)

// LACP / placeholder reportings.
const (
	LACPEnabled  = "enabled"
	LACPDisabled = "disabled"
	LACPNABlank  = "N/A"
	unknownField = "unknown"
)

// SwitchInfo is one row of the vswitches listing: a port group on a switch.
type SwitchInfo struct {
	Switch     string
	SwitchType string
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string
	Ports      int
	Used       int
}

// FetchSwitches returns every port group of every standard host vSwitch and
// distributed virtual switch. Identical switches on multiple hosts are
// aggregated across hosts. Fields the server does not populate degrade to
// unknown/N/A.
func FetchSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	agg := newSwitchAggregator()

	if err := addStandardSwitches(ctx, c, agg); err != nil {
		return nil, err
	}
	if err := addDistributedSwitches(ctx, c, agg); err != nil {
		return nil, err
	}
	return agg.rows(), nil
}

type switchAggKey struct{ switchName, switchType, portgroup string }

type switchAggEntry struct {
	vlans   []string
	uplinks []string
	lacp    string
	ports   int
	used    int
}

type switchAggregator struct {
	order []switchAggKey
	byKey map[switchAggKey]*switchAggEntry
}

func newSwitchAggregator() *switchAggregator {
	return &switchAggregator{byKey: map[switchAggKey]*switchAggEntry{}}
}

func (a *switchAggregator) add(switchName, switchType, portgroup, vlan, uplinks, lacp string, ports, used int) {
	k := switchAggKey{switchName, switchType, portgroup}
	e, ok := a.byKey[k]
	if !ok {
		e = &switchAggEntry{lacp: lacp}
		a.byKey[k] = e
		a.order = append(a.order, k)
	}
	e.ports += ports
	e.used += used
	if vlan != "" {
		e.vlans = appendUnique(e.vlans, vlan)
	}
	if uplinks != "" {
		e.uplinks = appendUnique(e.uplinks, uplinks)
	}
	if lacp == LACPEnabled {
		e.lacp = LACPEnabled
	}
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func (a *switchAggregator) rows() []SwitchInfo {
	out := make([]SwitchInfo, 0, len(a.order))
	for _, k := range a.order {
		e := a.byKey[k]
		out = append(out, SwitchInfo{
			Switch:     k.switchName,
			SwitchType: k.switchType,
			Portgroup:  k.portgroup,
			VLAN:       joinOrUnknown(e.vlans),
			Uplinks:    joinOrUnknown(e.uplinks),
			LACP:       e.lacp,
			Ports:      e.ports,
			Used:       e.used,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Switch != out[j].Switch {
			return out[i].Switch < out[j].Switch
		}
		return out[i].Portgroup < out[j].Portgroup
	})
	return out
}

func joinOrUnknown(parts []string) string {
	seen := map[string]struct{}{}
	var flat []string
	for _, p := range parts {
		for _, s := range strings.Split(p, ",") {
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				flat = append(flat, s)
			}
		}
	}
	if len(flat) == 0 {
		return unknownField
	}
	sort.Strings(flat)
	return strings.Join(flat, ",")
}

// addStandardSwitches folds every host's vSwitch0..N port groups into agg.
func addStandardSwitches(ctx context.Context, c *vim25.Client, agg *switchAggregator) error {
	var hosts []mo.HostSystem
	if err := retrieve(ctx, c, "HostSystem", []string{"name", "config.network"}, &hosts); err != nil {
		return err
	}
	for i := range hosts {
		h := &hosts[i]
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		network := h.Config.Network
		vswitches := map[string]*types.HostVirtualSwitch{}
		for j := range network.Vswitch {
			vs := &network.Vswitch[j]
			vswitches[vs.Name] = vs
		}
		for _, pg := range network.Portgroup {
			name := pg.Spec.Name
			sw := pg.Spec.VswitchName
			if sw == "" {
				sw = "N/A"
			}
			total, uplinks := 0, ""
			if vs, ok := vswitches[sw]; ok {
				total = int(vs.NumPorts)
				if total == 0 {
					total = int(vs.Spec.NumPorts)
				}
				uplinks = standardUplinks(vs)
			}
			used := len(pg.Port)
			if used > total {
				used = total
			}
			agg.add(sw, SwitchStandard, name, standardVLAN(pg.Spec.VlanId), uplinks, LACPNABlank, total, used)
		}
	}
	return nil
}

func standardVLAN(vlanID int32) string {
	if vlanID == 4095 {
		return "4095 (trunk)"
	}
	return strconv.FormatInt(int64(vlanID), 10)
}

func standardUplinks(vs *types.HostVirtualSwitch) string {
	var nics []string
	if vs.Spec.Policy != nil && vs.Spec.Policy.NicTeaming != nil && vs.Spec.Policy.NicTeaming.NicOrder != nil {
		nics = append(nics, vs.Spec.Policy.NicTeaming.NicOrder.ActiveNic...)
		nics = append(nics, vs.Spec.Policy.NicTeaming.NicOrder.StandbyNic...)
	}
	if len(nics) == 0 {
		if b, ok := vs.Spec.Bridge.(*types.HostVirtualSwitchBondBridge); ok {
			nics = append(nics, b.NicDevice...)
		}
	}
	if len(nics) == 0 {
		nics = append(nics, vs.Pnic...)
	}
	names := make([]string, 0, len(nics))
	for _, n := range nics {
		names = append(names, trimHostKeyPrefix(n))
	}
	return strings.Join(names, ",")
}

func trimHostKeyPrefix(s string) string {
	if i := strings.LastIndex(s, "-"); i >= 0 && strings.HasPrefix(s, "key-vim.host.") {
		return s[i+1:]
	}
	return s
}

// addDistributedSwitches folds every vDS port group into agg.
func addDistributedSwitches(ctx context.Context, c *vim25.Client, agg *switchAggregator) error {
	var dvss []mo.DistributedVirtualSwitch
	if err := retrieve(ctx, c, "DistributedVirtualSwitch", []string{"name", "config"}, &dvss); err != nil {
		return err
	}
	if len(dvss) == 0 {
		return nil
	}
	var dvpgs []mo.DistributedVirtualPortgroup
	if err := retrieve(ctx, c, "DistributedVirtualPortgroup", []string{"name", "key", "config"}, &dvpgs); err != nil {
		return err
	}

	// Host proxy switch data gives the physical NICs behind each vDS.
	uplinksByDvs, err := proxySwitchUplinks(ctx, c)
	if err != nil {
		return err
	}

	bySwitch := map[string][]mo.DistributedVirtualPortgroup{}
	pgKeysBySwitch := map[string][]string{}
	for _, pg := range dvpgs {
		if pg.Config.DistributedVirtualSwitch == nil {
			continue
		}
		k := pg.Config.DistributedVirtualSwitch.Value
		bySwitch[k] = append(bySwitch[k], pg)
		pgKeysBySwitch[k] = append(pgKeysBySwitch[k], pg.Config.Key)
	}

	for i := range dvss {
		dvs := &dvss[i]
		info, err := object.NewDistributedVirtualSwitch(c, dvs.Self).FetchDVPorts(ctx, &types.DistributedVirtualSwitchPortCriteria{
			PortgroupKey: pgKeysBySwitch[dvs.Self.Value],
			Inside:       types.NewBool(true),
			Connected:    types.NewBool(true),
		})
		if err != nil {
			// Connected-port counts are optional; degrade to zero used.
			info = nil
		}
		usedByPG := map[string]int{}
		for _, p := range info {
			usedByPG[p.PortgroupKey]++
		}
		lacp := dvsLACP(dvs.Config)
		uplinks := uplinksByDvs[dvs.Uuid]
		if uplinks == "" {
			uplinks = unknownField
		}
		for _, pg := range bySwitch[dvs.Self.Value] {
			ports := int(pg.Config.NumPorts)
			agg.add(dvs.Name, SwitchDistributed, pg.Config.Name, distributedVLAN(pg.Config.DefaultPortConfig), uplinks, lacp, ports, usedByPG[pg.Config.Key])
		}
	}
	return nil
}

func distributedVLAN(setting types.BaseDVPortSetting) string {
	ps, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || ps == nil || ps.Vlan == nil {
		return unknownField
	}
	switch vlan := ps.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		id := vlan.VlanId
		if id == 4095 {
			return "4095 (trunk)"
		}
		return strconv.FormatInt(int64(id), 10)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		return vlanRanges(vlan.VlanId) + " (trunk)"
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("private-vlan (%d)", vlan.PvlanId)
	default:
		return unknownField
	}
}

func vlanRanges(ranges []types.NumericRange) string {
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		if r.Start == r.End {
			parts = append(parts, strconv.FormatInt(int64(r.Start), 10))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
	}
	if len(parts) == 0 {
		return unknownField
	}
	return strings.Join(parts, ",")
}

func dvsLACP(cfg types.BaseDVSConfigInfo) string {
	vmw, ok := cfg.(*types.VMwareDVSConfigInfo)
	if !ok || vmw == nil {
		return LACPDisabled
	}
	for _, g := range vmw.LacpGroupConfig {
		switch strings.ToLower(g.Mode) {
		case "activeactive", "activepassive":
			return LACPEnabled
		}
	}
	return LACPDisabled
}

// proxySwitchUplinks maps each vDS UUID to the physical NICs / uplink port
// names configured for it on its hosts.
func proxySwitchUplinks(ctx context.Context, c *vim25.Client) (map[string]string, error) {
	var hosts []mo.HostSystem
	if err := retrieve(ctx, c, "HostSystem", []string{"name", "config.network"}, &hosts); err != nil {
		return nil, err
	}
	byDvs := map[string]map[string]struct{}{}
	for i := range hosts {
		h := &hosts[i]
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		for _, ps := range h.Config.Network.ProxySwitch {
			set, ok := byDvs[ps.DvsUuid]
			if !ok {
				set = map[string]struct{}{}
				byDvs[ps.DvsUuid] = set
			}
			for _, p := range ps.Pnic {
				set[p] = struct{}{}
			}
			for _, kv := range ps.UplinkPort {
				if kv.Value != "" {
					set[kv.Value] = struct{}{}
				}
			}
		}
	}
	out := map[string]string{}
	for uuid, set := range byDvs {
		names := make([]string, 0, len(set))
		for n := range set {
			names = append(names, n)
		}
		sort.Strings(names)
		out[uuid] = strings.Join(names, ",")
	}
	return out, nil
}

// FetchVMsByPortgroup returns the VMs with a network adapter attached to the
// named port group. Works for standard and distributed port groups.
func FetchVMsByPortgroup(ctx context.Context, c *vim25.Client, portgroup string) ([]VMInfo, error) {
	if strings.TrimSpace(portgroup) == "" {
		return nil, fmt.Errorf("portgroup name must not be empty")
	}
	dvpgs, err := distributedPortgroups(ctx, c)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, pg := range dvpgs {
		known[pg.Config.Name] = true
	}
	standard, err := standardPortgroupNames(ctx, c)
	if err != nil {
		return nil, err
	}
	for name := range standard {
		known[name] = true
	}
	if !known[portgroup] {
		return nil, fmt.Errorf("portgroup %q not found: no standard or distributed port group with that name", portgroup)
	}

	keyToName := map[string]string{}
	for _, pg := range dvpgs {
		keyToName[pg.Config.Key] = pg.Config.Name
	}
	vms, err := fetchVMsWithDevices(ctx, c)
	if err != nil {
		return nil, err
	}
	var out []VMInfo
	for _, vm := range vms {
		for _, name := range vmEthernetBackings(vm, keyToName) {
			if name == portgroup {
				out = append(out, collectVMs([]mo.VirtualMachine{vm})[0])
				break
			}
		}
	}
	sortVMs(out)
	return out, nil
}

func distributedPortgroups(ctx context.Context, c *vim25.Client) ([]mo.DistributedVirtualPortgroup, error) {
	var pgs []mo.DistributedVirtualPortgroup
	if err := retrieve(ctx, c, "DistributedVirtualPortgroup", []string{"name", "key", "config"}, &pgs); err != nil {
		return nil, err
	}
	return pgs, nil
}

func standardPortgroupNames(ctx context.Context, c *vim25.Client) (map[string]bool, error) {
	var hosts []mo.HostSystem
	if err := retrieve(ctx, c, "HostSystem", []string{"name", "config.network"}, &hosts); err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for i := range hosts {
		h := &hosts[i]
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		for _, pg := range h.Config.Network.Portgroup {
			names[pg.Spec.Name] = true
		}
	}
	return names, nil
}

package inventory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// SwitchInfo is one row of the vswitches subcommand: a single port group with
// its owning switch's attributes.
type SwitchInfo struct {
	Switch     string
	SwitchType string // "standard" | "distributed"
	PortGroup  string
	Vlan       string
	Uplinks    []string
	LACP       string // "enabled" | "disabled" | "N/A" (standard switches)
	Ports      int32
	Used       int32
}

// ListSwitches returns every standard host vSwitch and every distributed
// switch in the inventory, one row per port group, sorted by switch then by
// port group.
func ListSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	std, err := standardSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	dst, err := distributedSwitches(ctx, c)
	if err != nil {
		return nil, err
	}

	out := append(std, dst...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Switch != out[j].Switch {
			return out[i].Switch < out[j].Switch
		}
		return out[i].PortGroup < out[j].PortGroup
	})
	return out, nil
}

// standardSwitches walks every host's network config. A host may appear
// multiple times in the inventory; identical rows are emitted once.
func standardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	var refs []types.ManagedObjectReference
	err := withDatacenters(ctx, c, func(f *find.Finder) error {
		hosts, err := f.HostSystemList(ctx, "*")
		if err != nil {
			return err
		}
		for _, h := range hosts {
			refs = append(refs, h.Reference())
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find hosts: %w", err)
	}
	refs = dedupeRefs(refs)

	var mes []mo.HostSystem
	err = property.DefaultCollector(c).Retrieve(ctx, refs, []string{"name", "config.network"}, &mes)
	if err != nil {
		return nil, fmt.Errorf("retrieve host network config: %w", err)
	}

	var out []SwitchInfo
	seen := make(map[string]bool)
	for i := range mes {
		h := &mes[i]
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		net := h.Config.Network
		for _, vsw := range net.Vswitch {
			used := vsw.NumPorts - vsw.NumPortsAvailable
			if used < 0 {
				used = 0
			}
			for _, pg := range net.Portgroup {
				if pg.Vswitch != vsw.Key {
					continue
				}
				row := SwitchInfo{
					Switch:     vsw.Name,
					SwitchType: "standard",
					PortGroup:  pg.Spec.Name,
					Vlan:       standardVLAN(pg.Spec.VlanId),
					Uplinks:    append([]string(nil), vsw.Pnic...),
					LACP:       "N/A",
					Ports:      vsw.NumPorts,
					Used:       used,
				}
				key := strings.Join([]string{row.Switch, row.PortGroup, row.Vlan,
					strings.Join(row.Uplinks, ","), row.LACP,
					strconv.Itoa(int(row.Ports)), strconv.Itoa(int(row.Used))}, "|")
				if !seen[key] {
					seen[key] = true
					out = append(out, row)
				}
			}
		}
	}
	return out, nil
}

// standardVLAN renders a standard port group's vlanId. vim uses -1 for
// untagged and 4095 for trunk (all VLANs).
func standardVLAN(id int32) string {
	switch id {
	case -1:
		return "untagged"
	case 4095:
		return "trunk"
	default:
		return strconv.Itoa(int(id))
	}
}

// distributedSwitches lists every distributed switch plus its port groups.
func distributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	pc := property.DefaultCollector(c)

	var dvsRefs, pgRefs []types.ManagedObjectReference
	err := withDatacenters(ctx, c, func(f *find.Finder) error {
		nets, err := f.NetworkList(ctx, "*")
		if err != nil {
			return err
		}
		for _, n := range nets {
			switch r := n.Reference(); r.Type {
			case "DistributedVirtualSwitch", "VmwareDistributedVirtualSwitch":
				dvsRefs = append(dvsRefs, r)
			case "DistributedVirtualPortgroup":
				pgRefs = append(pgRefs, r)
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find distributed switches: %w", err)
	}
	dvsRefs = dedupeRefs(dvsRefs)
	pgRefs = dedupeRefs(pgRefs)

	var dvses []mo.DistributedVirtualSwitch
	if len(dvsRefs) > 0 {
		err = pc.Retrieve(ctx, dvsRefs,
			[]string{"name", "config.uplinkPortPolicy", "config.lacpGroupConfig"}, &dvses)
		if err != nil {
			return nil, fmt.Errorf("retrieve distributed switch properties: %w", err)
		}
	}

	dvsByName := make(map[string]switchMeta, len(dvses))
	for i := range dvses {
		meta := switchMeta{LACP: "disabled"}
		if cfg, ok := dvses[i].Config.(*types.VMwareDVSConfigInfo); ok {
			if pol, ok := cfg.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok {
				meta.Uplinks = append(meta.Uplinks, pol.UplinkPortName...)
			}
			if len(cfg.LacpGroupConfig) > 0 {
				meta.LACP = "enabled"
			}
		}
		meta.dedupe()
		dvsByName[dvses[i].Name] = meta
	}

	var mpgs []mo.DistributedVirtualPortgroup
	if len(pgRefs) > 0 {
		err = pc.Retrieve(ctx, pgRefs, []string{"name", "config", "portKeys"}, &mpgs)
		if err != nil {
			return nil, fmt.Errorf("retrieve distributed port group properties: %w", err)
		}
	}

	out := make([]SwitchInfo, 0, len(mpgs))
	for i := range mpgs {
		pg := &mpgs[i]
		row := SwitchInfo{
			SwitchType: "distributed",
			PortGroup:  pg.Name,
			Vlan:       distributedVLAN(&pg.Config),
			Ports:      pg.Config.NumPorts,
			Used:       int32(len(pg.PortKeys)),
		}
		if row.Used > row.Ports {
			row.Used = row.Ports
		}
		if owner := pg.Config.DistributedVirtualSwitch; owner != nil {
			name := dvsName(ctx, pc, owner)
			if meta, ok := dvsByName[name]; ok {
				row.Switch = name
				row.Uplinks = meta.Uplinks
				row.LACP = meta.LACP
			}
		}
		if row.Switch == "" {
			// A port group whose owning switch we could not resolve: degrade
			// instead of dropping the row or fabricating an owner.
			row.Switch = "unknown"
			row.LACP = "N/A"
		}
		out = append(out, row)
	}
	return out, nil
}

type switchMeta struct {
	Uplinks []string
	LACP    string
}

func (m *switchMeta) dedupe() {
	m.Uplinks = uniqueSorted(m.Uplinks)
}

// distributedVLAN renders the VLAN of a distributed port group. The vlan spec
// may describe a single ID, a trunk range, a private VLAN, untagged, or be
// entirely absent — each renders distinctly rather than showing one number.
func distributedVLAN(cfg *types.DVPortgroupConfigInfo) string {
	setting, ok := cfg.DefaultPortConfig.(*types.VMwareDVSPortSetting)
	if !ok || setting == nil || setting.Vlan == nil {
		return "untagged"
	}
	switch v := setting.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.Itoa(int(v.VlanId))
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		ranges := make([]string, 0, len(v.VlanId))
		for _, r := range v.VlanId {
			if r.Start == r.End {
				ranges = append(ranges, strconv.Itoa(int(r.Start)))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		sort.Strings(ranges)
		return strings.Join(ranges, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan-%d", v.PvlanId)
	default:
		return "unknown"
	}
}

func dvsName(ctx context.Context, pc *property.Collector, ref *types.ManagedObjectReference) string {
	var dvs mo.DistributedVirtualSwitch
	if err := pc.RetrieveOne(ctx, *ref, []string{"name"}, &dvs); err != nil {
		return ""
	}
	return dvs.Name
}

func uniqueSorted(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	var prev string
	for i, s := range in {
		if s == "" || (i > 0 && s == prev) {
			continue
		}
		out = append(out, s)
		prev = s
	}
	return out
}

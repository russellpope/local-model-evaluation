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

// SwitchInfo is one row of the `vswitches` report. Rows are port groups;
// PORTS/USED describe the parent switch. LACP applies to distributed
// switches only and is reported as "N/A" for standard ones.
type SwitchInfo struct {
	Switch     string
	SwitchType string // "standard" or "distributed"
	PortGroup  string
	VLAN       string
	Uplinks    string
	LACP       string // "enabled", "disabled", or "N/A"
	TotalPorts int32
	UsedPorts  int32
}

const (
	typeStandard    = "standard"
	typeDistributed = "distributed"
)

// ListSwitches returns every standard and distributed virtual switch with
// their port groups, sorted by switch name then port group name.
func ListSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	std, err := listStandardSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	dist, err := listDistributedSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	out := append(std, dist...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Switch != out[j].Switch {
			return out[i].Switch < out[j].Switch
		}
		return out[i].PortGroup < out[j].PortGroup
	})
	return out, nil
}

func listStandardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	v, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder,
		[]string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("create host container view: %w", err)
	}
	defer func() { _ = v.Destroy(context.Background()) }()

	var hosts []mo.HostSystem
	if err := v.Retrieve(ctx, []string{"HostSystem"}, []string{"config.network"}, &hosts); err != nil {
		return nil, fmt.Errorf("retrieve host networks: %w", err)
	}

	var out []SwitchInfo
	// Hosts commonly repeat the same standard switch configuration under one
	// name (e.g. every host has "vSwitch0"); since the report has no host
	// column, identical rows are collapsed to one.
	seen := make(map[string]bool)
	for _, h := range hosts {
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		network := h.Config.Network
		for _, vs := range network.Vswitch {
			uplinks := standardUplinks(vs)
			used := UsedOf(int64(vs.NumPorts), int64(vs.NumPortsAvailable))
			rows := 0
			for _, pg := range network.Portgroup {
				if pg.Spec.VswitchName != vs.Name {
					continue
				}
				row := SwitchInfo{
					Switch:     vs.Name,
					SwitchType: typeStandard,
					PortGroup:  pg.Spec.Name,
					VLAN:       vlanString(pg.Spec.VlanId),
					Uplinks:    uplinks,
					LACP:       "N/A",
					TotalPorts: vs.NumPorts,
					UsedPorts:  int32(used),
				}
				key := row.Switch + "|" + row.PortGroup + "|" + row.VLAN + "|" + row.Uplinks
				if seen[key] {
					continue
				}
				seen[key] = true
				rows++
				out = append(out, row)
			}
			if rows == 0 {
				row := SwitchInfo{
					Switch:     vs.Name,
					SwitchType: typeStandard,
					PortGroup:  "-",
					VLAN:       "-",
					Uplinks:    uplinks,
					LACP:       "N/A",
					TotalPorts: vs.NumPorts,
					UsedPorts:  int32(used),
				}
				key := row.Switch + "|" + row.PortGroup + "|" + row.VLAN + "|" + row.Uplinks
				if !seen[key] {
					seen[key] = true
					out = append(out, row)
				}
			}
		}
	}
	return out, nil
}

// standardUplinks renders the physical NICs backing a standard vSwitch.
func standardUplinks(vs types.HostVirtualSwitch) string {
	var nics []string
	if vs.Spec.Policy != nil && vs.Spec.Policy.NicTeaming != nil && vs.Spec.Policy.NicTeaming.NicOrder != nil {
		nics = append(nics, vs.Spec.Policy.NicTeaming.NicOrder.ActiveNic...)
		nics = append(nics, vs.Spec.Policy.NicTeaming.NicOrder.StandbyNic...)
	}
	if len(nics) == 0 {
		if bridge, ok := vs.Spec.Bridge.(*types.HostVirtualSwitchBondBridge); ok {
			nics = append(nics, bridge.NicDevice...)
		}
	}
	if len(nics) == 0 {
		for _, p := range vs.Pnic {
			if i := strings.LastIndex(p, "-"); i >= 0 {
				p = p[i+1:]
			}
			nics = append(nics, p)
		}
	}
	if len(nics) == 0 {
		return "-"
	}
	return strings.Join(nics, ",")
}

// vlanString renders a standard port group VLAN ID; 4095 means trunk.
func vlanString(id int32) string {
	if id == 4095 {
		return "trunk"
	}
	return strconv.FormatInt(int64(id), 10)
}

func listDistributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	// Real vCenter reports vDS objects as "VmwareDistributedVirtualSwitch";
	// govmomi's simulator registers them as "DistributedVirtualSwitch".
	dvsTypes := []string{"VmwareDistributedVirtualSwitch", "DistributedVirtualSwitch"}
	v, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder, dvsTypes, true)
	if err != nil {
		return nil, fmt.Errorf("create distributed switch container view: %w", err)
	}
	defer func() { _ = v.Destroy(context.Background()) }()

	var switches []mo.DistributedVirtualSwitch
	if err := v.Retrieve(ctx, dvsTypes,
		[]string{"name", "summary", "config", "portgroup"}, &switches); err != nil {
		return nil, fmt.Errorf("retrieve distributed switches: %w", err)
	}
	if len(switches) == 0 {
		return nil, nil
	}

	pc := property.DefaultCollector(c)

	// Per-switch facts keyed by switch moref value.
	type dvsFacts struct {
		name    string
		total   int32
		lacp    string
		uplinks string
	}
	facts := make(map[string]*dvsFacts, len(switches))
	for _, sw := range switches {
		f := &dvsFacts{name: sw.Name, lacp: "disabled"}
		f.total = sw.Summary.NumPorts
		if cfg, ok := sw.Config.(*types.VMwareDVSConfigInfo); ok {
			if len(cfg.LacpGroupConfig) > 0 {
				f.lacp = "enabled"
			}
			if f.total == 0 {
				f.total = cfg.MaxPorts
			}
			names, err := morefNames(ctx, pc, cfg.UplinkPortgroup)
			if err != nil {
				return nil, fmt.Errorf("resolve uplink port groups for %s: %w", sw.Name, err)
			}
			f.uplinks = strings.Join(names, ",")
		}
		if f.uplinks == "" {
			f.uplinks = "-"
		}
		facts[sw.Reference().Value] = f
	}

	// Ports actually in use on a distributed switch: connected port group ports.
	portGroupsOf := make(map[string][]types.ManagedObjectReference, len(switches))

	pgv, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder,
		[]string{"DistributedVirtualPortgroup"}, true)
	if err != nil {
		return nil, fmt.Errorf("create port group container view: %w", err)
	}
	defer func() { _ = pgv.Destroy(context.Background()) }()

	var pgs []mo.DistributedVirtualPortgroup
	if err := pgv.Retrieve(ctx, []string{"DistributedVirtualPortgroup"},
		[]string{"name", "config", "portKeys"}, &pgs); err != nil {
		return nil, fmt.Errorf("retrieve distributed port groups: %w", err)
	}

	var out []SwitchInfo
	for _, pg := range pgs {
		swRef := pg.Config.DistributedVirtualSwitch
		if swRef == nil {
			continue
		}
		f, ok := facts[swRef.Value]
		if !ok {
			continue
		}
		portGroupsOf[swRef.Value] = append(portGroupsOf[swRef.Value], pg.Reference())
		total := pg.Config.NumPorts // per-port-group allocation when reported
		if total == 0 {
			total = f.total
		}
		used := int32(len(pg.PortKeys))
		if used > total && total > 0 {
			used = total
		}
		out = append(out, SwitchInfo{
			Switch:     f.name,
			SwitchType: typeDistributed,
			PortGroup:  pgName(pg),
			VLAN:       dvpgVLANString(pg.Config.DefaultPortConfig),
			Uplinks:    f.uplinks,
			LACP:       f.lacp,
			TotalPorts: total,
			UsedPorts:  used,
		})
	}

	for refValue, f := range facts {
		if _, seen := portGroupsOf[refValue]; !seen {
			out = append(out, SwitchInfo{
				Switch:     f.name,
				SwitchType: typeDistributed,
				PortGroup:  "-",
				VLAN:       "-",
				Uplinks:    f.uplinks,
				LACP:       f.lacp,
				TotalPorts: f.total,
			})
		}
	}
	return out, nil
}

func pgName(pg mo.DistributedVirtualPortgroup) string {
	if pg.Config.Name != "" {
		return pg.Config.Name
	}
	return pg.Name
}

// morefNames resolves managed object references to their names.
func morefNames(ctx context.Context, pc *property.Collector, refs []types.ManagedObjectReference) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	var objs []mo.Network
	if err := pc.Retrieve(ctx, refs, []string{"name"}, &objs); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(objs))
	for _, o := range objs {
		names = append(names, o.Name)
	}
	sort.Strings(names)
	return names, nil
}

// dvpgVLANString renders a distributed port group's default VLAN setting:
// a single ID, trunk ranges, or a private VLAN.
func dvpgVLANString(def types.BaseDVPortSetting) string {
	s, ok := def.(*types.VMwareDVSPortSetting)
	if !ok || s == nil || s.Vlan == nil {
		return "unknown"
	}
	switch v := s.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.FormatInt(int64(v.VlanId), 10)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		ranges := make([]string, 0, len(v.VlanId))
		for _, r := range v.VlanId {
			ranges = append(ranges, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
		return "trunk " + strings.Join(ranges, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan %d", v.PvlanId)
	default:
		return "unknown"
	}
}

// ListPortGroupVMs returns the VMs attached to the named port group. Works
// for both standard and distributed port groups because both are Network
// objects that VMs reference through their network list.
func ListPortGroupVMs(ctx context.Context, c *vim25.Client, name string) ([]VMInfo, error) {
	nv, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder,
		[]string{"Network"}, true)
	if err != nil {
		return nil, fmt.Errorf("create network container view: %w", err)
	}

	var nets []mo.Network
	err = nv.Retrieve(ctx, []string{"Network"}, []string{"name"}, &nets)
	destroyErr := nv.Destroy(context.Background())
	if err != nil {
		return nil, fmt.Errorf("retrieve networks: %w", err)
	}
	if destroyErr != nil {
		return nil, fmt.Errorf("clean up network view: %w", destroyErr)
	}

	var refs []types.ManagedObjectReference
	for _, n := range nets {
		if n.Name == name {
			refs = append(refs, n.Reference())
		}
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("port group %q not found in inventory; run 'vswitches' to list available port groups", name)
	}

	v, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder,
		[]string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create VM container view: %w", err)
	}
	defer func() { _ = v.Destroy(context.Background()) }()

	var vms []mo.VirtualMachine
	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "network"}, &vms); err != nil {
		return nil, fmt.Errorf("retrieve VMs: %w", err)
	}

	want := make(map[string]bool, len(refs))
	for _, r := range refs {
		want[r.Value] = true
	}
	seen := make(map[string]bool)
	out := make([]VMInfo, 0)
	for _, vm := range vms {
		vmName := vm.Name
		if vmName == "" || seen[vmName] {
			continue
		}
		for _, n := range vm.Network {
			if want[n.Value] {
				seen[vmName] = true
				out = append(out, VMInfo{Name: vmName})
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

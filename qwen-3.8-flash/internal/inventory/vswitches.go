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

// Switch identifiers reported by the vswitches subcommand.
const (
	SwitchStandard    = "standard"
	SwitchDistributed = "distributed"

	lacpEnabled  = "enabled"
	lacpDisabled = "disabled"
	lacpNA       = "N/A"

	unknown = "unknown"
)

// SwitchInfo is one row of the vswitches table: a portgroup together with the
// state of the virtual switch that backs it.
type SwitchInfo struct {
	Switch     string
	SwitchType string // standard or distributed
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string // LACP applies to distributed switches only
	Ports      int64
	Used       int64
}

// ListSwitches returns every standard and distributed portgroup with its
// switch's state, sorted by switch then portgroup name.
func ListSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	rows, err := standardSwitchRows(ctx, c)
	if err != nil {
		return nil, err
	}
	dvsRows, err := distributedSwitchRows(ctx, c)
	if err != nil {
		return nil, err
	}
	rows = append(rows, dvsRows...)

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Switch != rows[j].Switch {
			return rows[i].Switch < rows[j].Switch
		}
		return rows[i].Portgroup < rows[j].Portgroup
	})
	return rows, nil
}

// standardSwitchRows walks every host's network config, aggregating identical
// vSwitch/portgroup combinations that repeat across hosts in a cluster.
func standardSwitchRows(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	refs, err := listKind(ctx, c, "HostSystem")
	if err != nil {
		return nil, err
	}
	var hosts []mo.HostSystem
	if err := collect(ctx, c, refs, []string{"name", "config"}, &hosts); err != nil {
		return nil, fmt.Errorf("list host networks: %w", err)
	}

	index := make(map[string]int)
	var rows []SwitchInfo

	for i := range hosts {
		h := &hosts[i]
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		net := h.Config.Network
		vswitches := make(map[string]types.HostVirtualSwitch, len(net.Vswitch))
		for _, vsw := range net.Vswitch {
			vswitches[vsw.Name] = vsw
		}
		for _, pg := range net.Portgroup {
			switchName := pg.Spec.VswitchName
			vsw, ok := vswitches[switchName]
			if !ok {
				switchName = unknown
			}
			key := "std|" + switchName + "|" + pg.Spec.Name
			if at, seen := index[key]; seen {
				if ok {
					rows[at].Uplinks = mergeUplinks(rows[at].Uplinks, pnicNames(vsw.Pnic))
					if total, used, derive := switchPortUsage(vsw); derive {
						if total > rows[at].Ports {
							rows[at].Ports = total
							rows[at].Used = used
						}
					}
				}
				continue
			}
			row := SwitchInfo{
				Switch:     switchName,
				SwitchType: SwitchStandard,
				Portgroup:  pg.Spec.Name,
				VLAN:       strconv.FormatInt(int64(pg.Spec.VlanId), 10),
				LACP:       lacpNA, // LACP is a distributed-switch feature
			}
			if ok {
				row.Uplinks = pnicNames(vsw.Pnic)
				if total, used, derive := switchPortUsage(vsw); derive {
					row.Ports, row.Used = total, used
				} else {
					row.Used = int64(len(pg.Port))
				}
			} else {
				row.Uplinks = unknown
				row.Used = int64(len(pg.Port))
			}
			index[key] = len(rows)
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// switchPortUsage derives a standard vSwitch's total and in-use port counts
// from its runtime state: used = total - available.
func switchPortUsage(vsw types.HostVirtualSwitch) (total, used int64, ok bool) {
	total = int64(vsw.NumPorts)
	if total == 0 && vsw.Spec.NumPorts > 0 {
		total = int64(vsw.Spec.NumPorts)
	}
	if vsw.NumPortsAvailable > 0 && total >= int64(vsw.NumPortsAvailable) {
		return total, total - int64(vsw.NumPortsAvailable), true
	}
	return total, 0, false
}

func pnicNames(keys []string) string {
	names := make([]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, strings.TrimPrefix(k, "key-vim.host.PhysicalNic-"))
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func mergeUplinks(a, b string) string {
	if a == b || b == "" || b == unknown {
		return a
	}
	if a == "" || a == unknown {
		return b
	}
	seen := make(map[string]bool)
	var all []string
	for _, part := range append(strings.Split(a, ","), strings.Split(b, ",")...) {
		if part != "" && !seen[part] {
			seen[part] = true
			all = append(all, part)
		}
	}
	sort.Strings(all)
	return strings.Join(all, ",")
}

// distributedSwitchRows returns one row per distributed portgroup.
func distributedSwitchRows(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	dvsRefs, err := listKind(ctx, c, "DistributedVirtualSwitch")
	if err != nil {
		return nil, fmt.Errorf("enumerate distributed switches: %w", err)
	}
	if len(dvsRefs) == 0 {
		return nil, nil
	}
	var switches []mo.DistributedVirtualSwitch
	if err := collect(ctx, c, dvsRefs, []string{"summary", "config", "portgroup"}, &switches); err != nil {
		return nil, fmt.Errorf("list distributed switches: %w", err)
	}

	var pgRefs []types.ManagedObjectReference
	for _, d := range switches {
		pgRefs = append(pgRefs, d.Portgroup...)
	}
	var pgs []mo.DistributedVirtualPortgroup
	if err := collect(ctx, c, pgRefs, []string{"name", "config", "portKeys"}, &pgs); err != nil {
		return nil, fmt.Errorf("list distributed portgroups: %w", err)
	}
	bySwitch := make(map[string][]*mo.DistributedVirtualPortgroup, len(switches))
	for i := range pgs {
		pg := &pgs[i]
		if pg.Config.DistributedVirtualSwitch != nil {
			key := pg.Config.DistributedVirtualSwitch.Value
			bySwitch[key] = append(bySwitch[key], pg)
		}
	}

	// vNICs attached per distributed portgroup — the API exposes no runtime
	// "ports used" counter for dv portgroups, so usage is derived from the
	// connected virtual NICs.
	usage, err := dvpgUsage(ctx, c)
	if err != nil {
		return nil, err
	}

	var rows []SwitchInfo
	for i := range switches {
		d := &switches[i]
		name := d.Summary.Name
		if cfg := d.Config.GetDVSConfigInfo(); cfg != nil && cfg.Name != "" {
			name = cfg.Name
		}
		uplinks := dvsUplinks(d.Config)
		lacp := dvsLacp(d.Config)

		for _, pg := range bySwitch[d.Self.Value] {
			rows = append(rows, SwitchInfo{
				Switch:     name,
				SwitchType: SwitchDistributed,
				Portgroup:  pg.Config.Name,
				VLAN:       dvsVlan(pg.Config.DefaultPortConfig),
				Uplinks:    uplinks,
				LACP:       lacp,
				Ports:      int64(pg.Config.NumPorts),
				Used:       usage[pg.Config.Key],
			})
		}
	}
	return rows, nil
}

// dvpgUsage counts virtual NICs connected to each distributed portgroup key.
func dvpgUsage(ctx context.Context, c *vim25.Client) (map[string]int64, error) {
	usage := make(map[string]int64)
	refs, err := listKind(ctx, c, "VirtualMachine")
	if err != nil {
		return nil, err
	}
	var vms []mo.VirtualMachine
	if err := collect(ctx, c, refs, []string{"config.hardware.device"}, &vms); err != nil {
		return nil, fmt.Errorf("list virtual machine devices: %w", err)
	}
	for i := range vms {
		for _, key := range connectedPortgroupKeys(&vms[i]) {
			usage[key]++
		}
	}
	return usage, nil
}

// connectedPortgroupKeys returns the portgroup identifiers a VM's virtual
// NICs are attached to: dvportgroup keys ("dvpg-N") for backed distributed
// ports, and standard device names for network backing.
func connectedPortgroupKeys(vm *mo.VirtualMachine) []string {
	if vm.Config == nil {
		return nil
	}
	var keys []string
	for _, device := range vm.Config.Hardware.Device {
		card, ok := device.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		switch backing := card.GetVirtualEthernetCard().Backing.(type) {
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			keys = append(keys, backing.Port.PortgroupKey)
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if backing.DeviceName != "" {
				keys = append(keys, backing.DeviceName)
			}
		}
	}
	return keys
}

func dvsUplinks(cfg types.BaseDVSConfigInfo) string {
	var policy types.BaseDVSUplinkPortPolicy
	switch c := cfg.(type) {
	case *types.DVSConfigInfo:
		policy = c.UplinkPortPolicy
	case *types.VMwareDVSConfigInfo:
		policy = c.UplinkPortPolicy
	}
	if named, ok := policy.(*types.DVSNameArrayUplinkPortPolicy); ok && len(named.UplinkPortName) > 0 {
		return strings.Join(named.UplinkPortName, ",")
	}
	return unknown
}

// dvsLacp reports LACP state. LACP is configured on VMware distributed
// switches only: the presence of an active LACP group means it is enabled.
func dvsLacp(cfg types.BaseDVSConfigInfo) string {
	switch c := cfg.(type) {
	case *types.VMwareDVSConfigInfo:
		if len(c.LacpGroupConfig) > 0 {
			return lacpEnabled
		}
		return lacpDisabled
	default:
		return lacpNA
	}
}

// dvsVlan renders a distributed portgroup's VLAN configuration: a single ID,
// a trunk range list, or a private-VLAN type.
func dvsVlan(portConfig types.BaseDVPortSetting) string {
	setting, ok := portConfig.(*types.VMwareDVSPortSetting)
	if !ok || setting == nil || setting.Vlan == nil {
		return unknown
	}
	switch v := setting.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return strconv.FormatInt(int64(v.VlanId), 10)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(v.VlanId) == 0 {
			return "trunk"
		}
		parts := make([]string, 0, len(v.VlanId))
		for _, r := range v.VlanId {
			if r.Start == r.End {
				parts = append(parts, strconv.FormatInt(int64(r.Start), 10))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		return strings.Join(parts, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("private-vlan:%d", v.PvlanId)
	default:
		return unknown
	}
}

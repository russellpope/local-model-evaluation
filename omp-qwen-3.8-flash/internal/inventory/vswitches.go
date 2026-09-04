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

// Switch type labels.
const (
	SwitchStandard    = "standard"
	SwitchDistributed = "distributed"
)

// LACP report values.
const (
	LACPEnabled  = "enabled"
	LACPDisabled = "disabled"
	LACPNA       = "N/A"
)

// Switches returns every port group on every virtual switch — standard
// (host vSwitch) and distributed (vDS) — sorted by switch then port group.
// Fields the API does not populate degrade to "unknown"/"N/A" rather than
// dropping rows.
func Switches(ctx context.Context, c *vim25.Client) ([]PortgroupInfo, error) {
	var out []PortgroupInfo

	std, err := standardSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	out = append(out, std...)

	dist, err := distributedSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	out = append(out, dist...)

	sort.Slice(out, func(i, j int) bool {
		if out[i].Switch != out[j].Switch {
			return out[i].Switch < out[j].Switch
		}
		return out[i].Portgroup < out[j].Portgroup
	})
	return out, nil
}

// --- standard switches -----------------------------------------------------

func standardSwitches(ctx context.Context, c *vim25.Client) ([]PortgroupInfo, error) {
	var hosts []mo.HostSystem
	if err := retrieveAll(ctx, c, "HostSystem",
		[]string{"name", "config.network"}, &hosts); err != nil {
		return nil, fmt.Errorf("list hosts for switches: %w", err)
	}

	// Deduplicate switches that appear identically on multiple hosts (same
	// cluster shared vSwitch config), keyed by switch+portgroup.
	seen := map[string]bool{}
	var out []PortgroupInfo

	for i := range hosts {
		if hosts[i].Config == nil {
			continue
		}
		net := hosts[i].Config.Network
		if net == nil {
			continue
		}
		pnicNames := map[string]string{} // pnic key -> device name
		for _, p := range net.Pnic {
			pnicNames[p.Key] = p.Device
		}
		pgByKey := map[string]types.HostPortGroup{}
		for _, pg := range net.Portgroup {
			pgByKey[pg.Key] = pg
		}

		for _, vsw := range net.Vswitch {
			// Spec: used ports = total - available, from the host's
			// per-switch runtime counters. Fall back to counting attached
			// ports when the host does not report available ports.
			ports := vsw.NumPorts
			used := int32(0)
			if vsw.NumPortsAvailable > 0 && vsw.NumPortsAvailable <= vsw.NumPorts {
				used = vsw.NumPorts - vsw.NumPortsAvailable
			} else {
				for _, pgKey := range vsw.Portgroup {
					if pg, ok := pgByKey[pgKey]; ok {
						used += int32(len(pg.Port))
					}
				}
			}
			uplinks := uplinkDevices(vsw.Pnic, pnicNames)
			for _, pgKey := range vsw.Portgroup {
				pg, ok := pgByKey[pgKey]
				name := pg.Spec.Name
				if !ok || name == "" {
					name = pgShortName(pgKey)
				}
				key := "std\x00" + vsw.Name + "\x00" + name
				if seen[key] {
					continue
				}
				seen[key] = true
				vlan := "unknown"
				if ok {
					vlan = stdVLAN(pg.Spec.VlanId)
				}
				out = append(out, PortgroupInfo{
					Switch:     vsw.Name,
					SwitchType: SwitchStandard,
					Portgroup:  name,
					VLAN:       vlan,
					Uplinks:    uplinks,
					LACP:       LACPNA, // LACP is a distributed-switch feature
					Ports:      ports,
					Used:       used,
				})
			}
		}
	}
	return out, nil
}

// pgShortName strips the "key-vim.host.PortGroup-" prefix used by the
// HostSystem virtualSwitch portgroup key list.
func pgShortName(key string) string {
	const prefix = "key-vim.host.PortGroup-"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return key
}

func stdVLAN(vlanID int32) string {
	switch {
	case vlanID == 0:
		return "0 (native)"
	case vlanID == 4095:
		return "4095 (trunk)"
	case vlanID == 4096:
		return "private-vlan"
	default:
		return strconv.FormatInt(int64(vlanID), 10)
	}
}

// uplinkDevices maps pnic keys to device names; unknown keys pass through.
func uplinkDevices(pnics []string, byKey map[string]string) string {
	if len(pnics) == 0 {
		return "none"
	}
	devs := make([]string, 0, len(pnics))
	for _, p := range pnics {
		if d, ok := byKey[p]; ok {
			devs = append(devs, d)
		} else {
			devs = append(devs, p)
		}
	}
	sort.Strings(devs)
	return strings.Join(devs, ",")
}

// --- distributed switches --------------------------------------------------

func distributedSwitches(ctx context.Context, c *vim25.Client) ([]PortgroupInfo, error) {
	var switches []mo.DistributedVirtualSwitch
	if err := retrieveAll(ctx, c, "DistributedVirtualSwitch",
		[]string{"name", "uuid", "summary", "config", "capability", "portgroup"}, &switches); err != nil {
		return nil, fmt.Errorf("list distributed switches: %w", err)
	}

	var out []PortgroupInfo
	for i := range switches {
		dsw := &switches[i]
		var cfg *types.DVSConfigInfo
		if dsw.Config != nil {
			cfg = dsw.Config.GetDVSConfigInfo()
		}
		if cfg == nil {
			cfg = &types.DVSConfigInfo{}
		}

		switchPorts := cfg.NumPorts
		lacp := LACPDisabled
		var lacpGroups []types.VMwareDvsLacpGroupConfig
		if vm, ok := dsw.Config.(*types.VMwareDVSConfigInfo); ok {
			lacpGroups = vm.LacpGroupConfig
			for _, g := range vm.LacpGroupConfig {
				if !strings.EqualFold(g.Mode, "disabled") && (g.Name != "" || g.UplinkNum > 0) {
					lacp = LACPEnabled
					break
				}
			}
		}

		uplinks := dvsUplinks(cfg, lacpGroups)

		// Port groups: prefer DVS.Summary.PortgroupName (names), but resolve
		// each group's own config for VLAN/ports when retrievable.
		pgRefs := dsw.Portgroup
		for _, ref := range pgRefs {
			var pg mo.DistributedVirtualPortgroup
			if err := retrieveOneAll(ctx, c, ref, []string{"name", "key", "config", "portKeys"}, &pg); err != nil {
				return nil, fmt.Errorf("retrieve distributed portgroup %s: %w", ref.Value, err)
			}
			if isUplinkPortgroup(&pg) {
				continue
			}
			pci := pg.Config
			ports := pci.NumPorts
			if ports == 0 {
				ports = switchPorts
			}
			pgLACP := lacp
			if ps, ok := pci.DefaultPortConfig.(*types.VMwareDVSPortSetting); ok &&
				ps.LacpPolicy != nil && ps.LacpPolicy.Enable != nil && ps.LacpPolicy.Enable.Value != nil && *ps.LacpPolicy.Enable.Value {
				pgLACP = LACPEnabled
			}
			used, err := dvsUsedPorts(ctx, c, dsw, &pg)
			if err != nil {
				// Connected-port counts are best-effort; degrade gracefully.
				used = 0
			}
			out = append(out, PortgroupInfo{
				Switch:     dsw.Name,
				SwitchType: SwitchDistributed,
				Portgroup:  pci.Name,
				VLAN:       dvsVLAN(pci.DefaultPortConfig),
				Uplinks:    uplinks,
				LACP:       pgLACP,
				Ports:      ports,
				Used:       used,
			})
		}
	}
	return out, nil
}

// isUplinkPortgroup reports whether pg is a switch uplink port group
// (DVS0-DVUplinks style), which should not be listed as a workload group.
func isUplinkPortgroup(pg *mo.DistributedVirtualPortgroup) bool {
	// vCenter names uplink portgroups "<switch>-DVUplinks-<moid>"; live
	// appliances also expose them with Config.Type "uplink" in some versions.
	return strings.Contains(strings.ToUpper(pg.Name), "UPLINK") ||
		strings.EqualFold(pg.Config.Type, "uplink")
}

// dvsUplinks lists the switch's uplink port names. vCenter reports them in
// the uplink port-group policy; when unset we fall back to LACP group uplink
// names; otherwise "unknown".
func dvsUplinks(cfg *types.DVSConfigInfo, lacpGroups []types.VMwareDvsLacpGroupConfig) string {
	if policy, ok := cfg.UplinkPortPolicy.(*types.DVSNameArrayUplinkPortPolicy); ok && len(policy.UplinkPortName) > 0 {
		return strings.Join(policy.UplinkPortName, ",")
	}
	var names []string
	for _, g := range lacpGroups {
		names = append(names, g.UplinkName...)
	}
	if len(names) > 0 {
		sort.Strings(names)
		return strings.Join(dedupe(names), ",")
	}
	return "unknown"
}

// dvsVLAN renders the distributed port group VLAN: a single ID, a trunk
// range, or a private-VLAN type. Missing config degrades to "unknown".
func dvsVLAN(setting types.BaseDVPortSetting) string {
	if setting == nil {
		return "unknown"
	}
	vm, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || vm.Vlan == nil {
		// Non-VMware port settings may carry a generic VLAN spec.
		return "unknown"
	}
	switch v := vm.Vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		return stdVLAN(v.VlanId)
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(v.VlanId) == 0 {
			return "trunk (empty)"
		}
		parts := make([]string, 0, len(v.VlanId))
		for _, r := range v.VlanId {
			if r.Start == r.End {
				parts = append(parts, strconv.FormatInt(int64(r.Start), 10))
			} else {
				parts = append(parts, fmt.Sprintf("%d-%d", r.Start, r.End))
			}
		}
		return "trunk " + strings.Join(parts, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("private-vlan %d", v.PvlanId)
	default:
		return "unknown"
	}
}

// dvsUsedPorts counts connected ports in one distributed port group via
// FetchDVPorts. vcsim does not populate port connectees, so a zero result is
// expected against the simulator and is not an error.
func dvsUsedPorts(ctx context.Context, c *vim25.Client, dsw *mo.DistributedVirtualSwitch, pg *mo.DistributedVirtualPortgroup) (int32, error) {
	client := object.NewDistributedVirtualSwitch(c, dsw.Self)
	connected := true
	ports, err := client.FetchDVPorts(ctx, &types.DistributedVirtualSwitchPortCriteria{
		PortgroupKey: []string{pg.Key},
		Connected:    &connected,
	})
	if err != nil {
		return 0, err
	}
	return int32(len(ports)), nil
}

func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

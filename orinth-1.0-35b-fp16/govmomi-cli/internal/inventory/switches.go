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
	mo "github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// SwitchInfo is the inventory record returned by ListSwitches. Each row
// corresponds to one port group on one switch (standard or distributed).
type SwitchInfo struct {
	Switch         string `json:"switch"`
	SwitchType     string `json:"switch_type"` // "standard" | "distributed"
	Host           string `json:"host"`        // ESXi host name (standard PGs only); empty for DVS
	PortGroup      string `json:"port_group"`
	VLAN           string `json:"vlan"`    // single ID, range, or empty
	Uplinks        string `json:"uplinks"` // comma-separated physical NIC names (standard); N/A for DVS
	LACP           string `json:"lacp"`    // "enabled" | "disabled" | "N/A"
	TotalPorts     int32  `json:"total_ports"`
	AvailablePorts int32  `json:"available_ports"`
	UsedPorts      int32  `json:"used_ports"`
	UsedPortsValid bool   `json:"used_ports_valid"` // false when USED cannot be derived (e.g. DVS)
}

// ListSwitches enumerates every standard vSwitch and distributed virtual switch
// in the inventory, returning one record per port group with its VLAN, uplinks,
// LACP status, and port counts. The result is sorted by (switch name, port
// group name) for deterministic tabular output.
func ListSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	var infos []SwitchInfo

	stdInfos, err := listStandardSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("listing standard switches: %w", err)
	}
	infos = append(infos, stdInfos...)

	dvsInfos, err := listDistributedSwitches(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("listing distributed switches: %w", err)
	}
	infos = append(infos, dvsInfos...)

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Switch != infos[j].Switch {
			return infos[i].Switch < infos[j].Switch
		}
		return infos[i].PortGroup < infos[j].PortGroup
	})

	return infos, nil
}

// listStandardSwitches walks every host in the inventory and reads its
// HostSystem.Config.Network to extract standard vSwitches and their port groups.
func listStandardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	hostRefs, err := findHostsInInventory(ctx, c)
	if err != nil {
		return nil, err
	}

	pc := property.DefaultCollector(c)
	var out []SwitchInfo

	for _, ref := range hostRefs {
		var hs mo.HostSystem
		if err := pc.RetrieveOne(ctx, ref, []string{"config.network", "name"}, &hs); err != nil {
			return nil, fmt.Errorf("reading host %s network config: %w", ref.String(), err)
		}

		netInfo := hs.Config.Network
		if netInfo == nil {
			continue
		}

		vsMap := make(map[string]*types.HostVirtualSwitch)
		for i := range netInfo.Vswitch {
			vs := &netInfo.Vswitch[i]
			vsMap[vs.Name] = vs
		}

		for _, pg := range netInfo.Portgroup {
			si := SwitchInfo{
				SwitchType: "standard",
				Host:       hs.Name,
				PortGroup:  pg.Spec.Name,
				VLAN:       formatStandardVLAN(pg.Spec.VlanId),
			}

			if vs, ok := vsMap[pg.Spec.VswitchName]; ok {
				si.Switch = vs.Name
				si.TotalPorts = vs.NumPorts
				si.AvailablePorts = vs.NumPortsAvailable
				si.UsedPorts = si.TotalPorts - si.AvailablePorts
				if si.UsedPorts < 0 {
					si.UsedPorts = 0
				}
				si.Uplinks = formatUplinks(vs)
				si.LACP = "N/A"
			} else {
				si.Switch = "(floating)"
				si.Uplinks = "N/A"
			}
			si.UsedPortsValid = true

			out = append(out, si)
		}
	}

	return out, nil
}

// listDistributedSwitches enumerates every DistributedVirtualSwitch in the
// inventory and extracts its port groups with VLAN, LACP, uplinks and port
// counts. Where vcsim does not expose these details fields degrade to "N/A" /
// 0 — callers should still see well-formed output without panics.
func listDistributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	m := view.NewManager(c)

	cv, err := m.CreateContainerView(
		ctx,
		c.ServiceContent.RootFolder,
		[]string{"DistributedVirtualSwitch"},
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("creating DVS container view: %w", err)
	}
	defer cv.Destroy(ctx)

	dvsRefs, err := cv.Find(ctx, []string{"DistributedVirtualSwitch"}, property.Match{"name": "*"})
	if err != nil {
		return nil, fmt.Errorf("finding distributed virtual switches: %w", err)
	}

	pc := property.DefaultCollector(c)

	var dvss []mo.DistributedVirtualSwitch
	if len(dvsRefs) > 0 {
		if err := pc.Retrieve(ctx, dvsRefs, []string{"name", "summary", "config", "portgroup"}, &dvss); err != nil {
			return nil, fmt.Errorf("batch retrieve DVS objects: %w", err)
		}
	}

	dvsByName := make(map[string]*mo.DistributedVirtualSwitch, len(dvss))
	for i := range dvss {
		if dvss[i].Name != "" {
			dvsByName[dvss[i].Name] = &dvss[i]
		}
	}

	pgRefs := collectAllDVSportgroupRefs(dvss)
	type pgEntry struct {
		ref types.ManagedObjectReference
		obj mo.DistributedVirtualPortgroup
	}
	var allPgs []pgEntry
	if len(pgRefs) > 0 {
		var pgs []mo.DistributedVirtualPortgroup
		if err := pc.Retrieve(ctx, pgRefs, []string{"name", "config"}, &pgs); err != nil {
			return nil, fmt.Errorf("batch retrieve DVS port groups: %w", err)
		}
		for i := range pgs {
			allPgs = append(allPgs, pgEntry{ref: pgRefs[i], obj: pgs[i]})
		}
	}

	pgByRefValue := make(map[string]*mo.DistributedVirtualPortgroup, len(allPgs))
	for i := range allPgs {
		if allPgs[i].obj.Name != "" {
			pgByRefValue[allPgs[i].ref.Value] = &allPgs[i].obj
		}
	}

	var out []SwitchInfo
	for _, dvs := range dvss {
		dvsName := dvs.Name
		if dvsName == "" {
			continue
		}

		lacpStatus := lacpStatusFromConfig(dvs.Config)

		for _, pgRef := range dvs.Portgroup {
			pg, ok := pgByRefValue[pgRef.Value]
			if !ok {
				continue // skip unreadable port group
			}

			si := SwitchInfo{
				Switch:         dvsName,
				SwitchType:     "distributed",
				PortGroup:      pg.Name,
				VLAN:           formatDVSvVLAN(pg.Config.DefaultPortConfig),
				Uplinks:        "N/A", // uplinks are a DVS-level concept, not per-portgroup in vSphere API
				LACP:           lacpStatus,
				TotalPorts:     pg.Config.NumPorts,
				UsedPortsValid: false, // AvailablePorts is not exposed by the DVS portgroup API; never render Total-0 as if computed.
			}

			out = append(out, si)
		}
	}

	return out, nil
}

// collectAllDVSportgroupRefs flattens every port group reference across a slice
// of DistributedVirtualSwitches into a single ref slice suitable for batch Retrieve.
func collectAllDVSportgroupRefs(dvss []mo.DistributedVirtualSwitch) []types.ManagedObjectReference {
	var out []types.ManagedObjectReference
	for _, dvs := range dvss {
		out = append(out, dvs.Portgroup...)
	}
	return out
}

// formatStandardVLAN renders the VLAN ID for a standard port group. A value of
// 0 means no VLAN; values >= 4096 are the vSphere trunk sentinel rendered as
// "1-4094". Values in [1, 4094] are shown verbatim.
func formatStandardVLAN(id int32) string {
	if id == 0 {
		return ""
	}
	if id >= 4096 {
		return "1-4094"
	}
	return strconv.Itoa(int(id))
}

// formatDVSvVLAN extracts the VLAN representation from a DVS port group's
// default port config. The BaseDVPortSetting interface only exposes the
// non-VLAN fields; the VLAN lives on the VMware-specific extension type
// (VMwareDVSPortSetting) which is what vSphere actually returns for DVS port
// groups. We type-assert to reach it, falling back gracefully when the
// extension is absent (e.g. against vcsim).
func formatDVSvVLAN(cfg types.BaseDVPortSetting) string {
	if cfg == nil {
		return ""
	}

	wds, ok := cfg.(*types.VMwareDVSPortSetting)
	if !ok || wds == nil {
		return ""
	}

	vlan := wds.Vlan
	if vlan == nil {
		return ""
	}

	switch v := vlan.(type) {
	case *types.VmwareDistributedVirtualSwitchVlanIdSpec:
		if v.VlanId <= 0 {
			return ""
		}
		return strconv.Itoa(int(v.VlanId))
	case *types.VmwareDistributedVirtualSwitchTrunkVlanSpec:
		if len(v.VlanId) == 0 {
			return "1-4094" // default trunk range when none specified
		}
		var ranges []string
		for _, r := range v.VlanId {
			ranges = append(ranges, fmt.Sprintf("%d-%d", r.Start, r.End))
		}
		return joinStrings(ranges, ",")
	case *types.VmwareDistributedVirtualSwitchPvlanSpec:
		return fmt.Sprintf("pvlan(%d)", v.PvlanId)
	default:
		return ""
	}
}

// formatUplinks renders the uplink (physical NIC) names for a standard vSwitch.
func formatUplinks(vs *types.HostVirtualSwitch) string {
	if vs == nil || len(vs.Pnic) == 0 {
		return "N/A"
	}
	nics := make([]string, 0, len(vs.Pnic))
	for _, p := range vs.Pnic {
		nics = append(nics, stripPnicKey(p))
	}
	return joinStrings(nics, ",")
}

const pnicKeyPrefix = "key-vim.host.PhysicalNic-"

// stripPnicKey removes the MO key prefix from a physical NIC reference,
// yielding the bare device name (e.g. "vmnic0"). Pass-through when the
// prefix is absent.
func stripPnicKey(ref string) string {
	if strings.HasPrefix(ref, pnicKeyPrefix) {
		return ref[len(pnicKeyPrefix):]
	}
	return ref
}

// lacpStatusFromConfig inspects a DVS's config and reports whether LACP is
// enabled on any of its Link Aggregation Groups. Standard vSwitches do not
// support LACP so this returns "disabled" when cfg is nil or not a
// VMwareDVSConfigInfo.
func lacpStatusFromConfig(cfg types.BaseDVSConfigInfo) string {
	if cfg == nil {
		return "disabled"
	}

	vd, ok := cfg.(*types.VMwareDVSConfigInfo)
	if !ok || len(vd.LacpGroupConfig) == 0 {
		return "disabled"
	}

	return "enabled"
}

// joinStrings concatenates strings with a separator; the empty slice returns "".
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += sep + p
	}
	return out
}

// findHostsInInventory returns every HostSystem managed object reference under
// the root folder using a ContainerView for efficiency.
func findHostsInInventory(ctx context.Context, c *vim25.Client) ([]types.ManagedObjectReference, error) {
	m := view.NewManager(c)

	hcv, err := m.CreateContainerView(
		ctx,
		c.ServiceContent.RootFolder,
		[]string{"HostSystem"},
		true, // recursive: find hosts in clusters and folders
	)
	if err != nil {
		return nil, fmt.Errorf("creating host container view: %w", err)
	}
	defer hcv.Destroy(ctx)

	hostRefs, err := hcv.Find(ctx, []string{"HostSystem"}, property.Match{"name": "*"})
	if err != nil {
		return nil, fmt.Errorf("finding hosts: %w", err)
	}

	return hostRefs, nil
}

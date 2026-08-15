package inventory

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// SwitchInfo is one row of the vswitches table: one port group of one
// virtual switch (standard or distributed).
type SwitchInfo struct {
	Switch     string
	SwitchType string // "standard" or "distributed"
	Portgroup  string
	VLAN       string
	Uplinks    string
	LACP       string // "enabled", "disabled" or "N/A"
	Ports      int
	Used       int
}

func ListSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	var rows []SwitchInfo

	standard, err := standardSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	rows = append(rows, standard...)

	distributed, err := distributedSwitches(ctx, c)
	if err != nil {
		return nil, err
	}
	rows = append(rows, distributed...)

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Switch != rows[j].Switch {
			return rows[i].Switch < rows[j].Switch
		}
		return rows[i].Portgroup < rows[j].Portgroup
	})
	return rows, nil
}

func standardSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	refs, err := viewRefs(ctx, c, []string{"HostSystem"})
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	if len(refs) == 0 {
		return nil, nil
	}
	var objs []mo.HostSystem
	if err := property.DefaultCollector(c).Retrieve(ctx, refs,
		[]string{"name", "configManager.networkSystem"}, &objs); err != nil {
		return nil, fmt.Errorf("collect host network system references: %w", err)
	}

	netRefs := make([]types.ManagedObjectReference, 0, len(objs))
	for i := range objs {
		if ns := objs[i].ConfigManager.NetworkSystem; ns != nil {
			netRefs = append(netRefs, *ns)
		}
	}
	var netSystems []mo.HostNetworkSystem
	if len(netRefs) > 0 {
		if err := property.DefaultCollector(c).Retrieve(ctx, netRefs,
			[]string{"networkInfo"}, &netSystems); err != nil {
			return nil, fmt.Errorf("collect host network info: %w", err)
		}
	}

	var rows []SwitchInfo
	for i := range netSystems {
		info := netSystems[i].NetworkInfo
		if info == nil {
			continue
		}
		net := info

		vnicName := map[string]string{}
		for _, vnic := range net.Vnic {
			if vnic.Key != "" {
				vnicName[vnic.Key] = vnic.Device
			}
		}

		// A switch's port group list holds port group keys on a real
		// vCenter; vcsim references port groups by name. Match on both.
		pgByKey := map[string]types.HostPortGroup{}
		pgByName := map[string]types.HostPortGroup{}
		for _, pg := range net.Portgroup {
			pgByKey[pg.Key] = pg
			pgByName[pg.Spec.Name] = pg
		}

		for _, sw := range net.Vswitch {
			uplinks := make([]string, 0, len(sw.Pnic))
			for _, pnic := range sw.Pnic {
				if name, ok := vnicName[pnic]; ok {
					uplinks = append(uplinks, name)
				} else {
					uplinks = append(uplinks, pnic)
				}
			}

			used := UsedBytes(int64(sw.NumPorts), int64(sw.NumPortsAvailable))

			pgs := make([]types.HostPortGroup, 0, len(sw.Portgroup))
			seen := map[string]bool{}
			for _, ref := range sw.Portgroup {
				pg, ok := pgByKey[ref]
				if !ok {
					pg, ok = pgByName[ref]
				}
				if ok && !seen[pg.Key] {
					seen[pg.Key] = true
					pgs = append(pgs, pg)
				}
			}
			sort.Slice(pgs, func(i, j int) bool { return pgs[i].Spec.Name < pgs[j].Spec.Name })

			if len(pgs) == 0 {
				rows = append(rows, SwitchInfo{
					Switch:     sw.Name,
					SwitchType: "standard",
					Portgroup:  "-",
					VLAN:       "-",
					Uplinks:    joinOrUnknown(uplinks),
					LACP:       "N/A",
					Ports:      int(sw.NumPorts),
					Used:       int(used),
				})
				continue
			}

			for _, pg := range pgs {
				rows = append(rows, SwitchInfo{
					Switch:     sw.Name,
					SwitchType: "standard",
					Portgroup:  pg.Spec.Name,
					VLAN:       strconv.FormatInt(int64(pg.Spec.VlanId), 10),
					Uplinks:    joinOrUnknown(uplinks),
					LACP:       "N/A",
					Ports:      int(sw.NumPorts),
					Used:       int(used),
				})
			}
		}
	}
	return rows, nil
}

func distributedSwitches(ctx context.Context, c *vim25.Client) ([]SwitchInfo, error) {
	v, err := rootView(ctx, c, []string{"DistributedVirtualSwitch", "VmwareDistributedVirtualSwitch"})
	if err != nil {
		return nil, err
	}
	defer func() { _ = v.Destroy(ctx) }()

	var dvss []mo.DistributedVirtualSwitch
	if err := v.Retrieve(ctx, []string{"DistributedVirtualSwitch"},
		[]string{"name", "config", "portgroup"}, &dvss); err != nil {
		return nil, fmt.Errorf("collect distributed virtual switch properties: %w", err)
	}
	if len(dvss) == 0 {
		return nil, nil
	}

	// Resolve uplink port group and port group names in one round trip.
	pgRefs := make([]types.ManagedObjectReference, 0)
	for i := range dvss {
		pgRefs = append(pgRefs, dvss[i].Portgroup...)
		if cfg, ok := dvss[i].Config.(*types.VMwareDVSConfigInfo); ok {
			pgRefs = append(pgRefs, cfg.UplinkPortgroup...)
		} else if cfg := dvss[i].Config.GetDVSConfigInfo(); cfg != nil {
			pgRefs = append(pgRefs, cfg.UplinkPortgroup...)
		}
	}
	nameByRef := map[string]string{}
	if len(pgRefs) > 0 {
		var pgs []mo.Network
		if err := property.DefaultCollector(c).Retrieve(ctx, pgRefs,
			[]string{"name"}, &pgs); err != nil {
			return nil, fmt.Errorf("collect port group names: %w", err)
		}
		for i := range pgs {
			nameByRef[pgs[i].Self.Value] = pgs[i].Name
		}
	}

	var rows []SwitchInfo
	for i := range dvss {
		dvs := &dvss[i]

		if dvs.Config == nil {
			continue
		}
		cfg := dvs.Config.GetDVSConfigInfo()
		if cfg == nil {
			continue
		}

		uplinks := make([]string, 0, len(cfg.UplinkPortgroup))
		for _, ref := range cfg.UplinkPortgroup {
			if name, ok := nameByRef[ref.Value]; ok {
				uplinks = append(uplinks, name)
			}
		}

		lacp := "disabled"
		if vmware, ok := dvs.Config.(*types.VMwareDVSConfigInfo); ok {
			for _, group := range vmware.LacpGroupConfig {
				if strings.EqualFold(group.Mode, "enabled") {
					lacp = "enabled"
					break
				}
			}
		} else {
			lacp = "N/A"
		}

		total := int(cfg.MaxPorts)
		used := int(cfg.NumPorts)
		if total < used {
			total = used
		}

		pgs := make([]mo.DistributedVirtualPortgroup, 0, len(dvs.Portgroup))
		if len(dvs.Portgroup) > 0 {
			pgRefs := make([]types.ManagedObjectReference, 0, len(dvs.Portgroup))
			for _, ref := range dvs.Portgroup {
				pgRefs = append(pgRefs, ref)
			}
			if err := property.DefaultCollector(c).Retrieve(ctx, pgRefs,
				[]string{"name", "config"}, &pgs); err != nil {
				return nil, fmt.Errorf("collect distributed port groups: %w", err)
			}
		}
		sort.Slice(pgs, func(i, j int) bool { return pgs[i].Name < pgs[j].Name })

		if len(pgs) == 0 {
			rows = append(rows, SwitchInfo{
				Switch:     dvs.Name,
				SwitchType: "distributed",
				Portgroup:  "-",
				VLAN:       "-",
				Uplinks:    joinOrUnknown(uplinks),
				LACP:       lacp,
				Ports:      total,
				Used:       used,
			})
			continue
		}

		for j := range pgs {
			pg := &pgs[j]
			rows = append(rows, SwitchInfo{
				Switch:     dvs.Name,
				SwitchType: "distributed",
				Portgroup:  pg.Name,
				VLAN:       dvpgVLAN(pg.Config.DefaultPortConfig),
				Uplinks:    joinOrUnknown(uplinks),
				LACP:       lacp,
				Ports:      total,
				Used:       used,
			})
		}
	}
	return rows, nil
}

// dvpgVLAN renders the VLAN setting of a distributed port group. Single-VLAN
// port groups show the ID (0 means no VLAN). Trunk port groups show the VLAN
// range(s).
func dvpgVLAN(setting types.BaseDVPortSetting) string {
	vmware, ok := setting.(*types.VMwareDVSPortSetting)
	if !ok || vmware.Vlan == nil {
		return "0"
	}
	switch vlan := vmware.Vlan.(type) {
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
		return strings.Join(parts, ",")
	default:
		return "unknown"
	}
}

func joinOrUnknown(items []string) string {
	if len(items) == 0 {
		return "unknown"
	}
	return strings.Join(items, ",")
}

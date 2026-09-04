package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Switch kinds and LACP states as rendered in the vswitches table.
const (
	SwitchStandard    = "standard"
	SwitchDistributed = "distributed"

	LACPEnabled       = "enabled"
	LACPDisabled      = "disabled"
	LACPNotApplicable = "N/A"
)

// PortGroupInfo is one port group row of a switch.
type PortGroupInfo struct {
	Name string
	VLAN string
}

// SwitchInfo describes a virtual switch (standard host vSwitch or
// distributed switch) together with its port groups.
//
// For standard switches: TotalPorts/UsedPorts come from the host's reported
// numPorts/numPortsAvailable (used = total - available), LACP is N/A and
// Uplinks lists the physical NIC devices of the switch bridge.
//
// For distributed switches: TotalPorts prefers the switch config numPorts
// (falling back to the number of existing ports when the property is not
// populated), UsedPorts counts ports with a connectee as reported by
// FetchDVPorts, LACP reflects the configured LACP groups, and Uplinks lists
// the uplink port group names.
type SwitchInfo struct {
	Name       string
	Kind       string
	Uplinks    []string
	LACP       string
	TotalPorts int64
	UsedPorts  int64
	PortGroups []PortGroupInfo
}

// ListSwitches returns all virtual switches in the inventory, both standard
// (per-host) and distributed, sorted by switch name then kind.
func ListSwitches(ctx context.Context, c *govmomi.Client) ([]SwitchInfo, error) {
	v, err := newContainerView(ctx, c, []string{"HostSystem", "DistributedVirtualSwitch", "DistributedVirtualPortgroup"})
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts)
	if err != nil {
		return nil, fmt.Errorf("retrieve hosts: %w", err)
	}

	var dvss []mo.DistributedVirtualSwitch
	err = v.Retrieve(ctx, []string{"DistributedVirtualSwitch"}, []string{"name", "config", "portgroup"}, &dvss)
	if err != nil {
		return nil, fmt.Errorf("retrieve distributed switches: %w", err)
	}

	var dvpgs []mo.DistributedVirtualPortgroup
	err = v.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "key", "config"}, &dvpgs)
	if err != nil {
		return nil, fmt.Errorf("retrieve distributed port groups: %w", err)
	}
	dvpgByMoid := make(map[string]mo.DistributedVirtualPortgroup, len(dvpgs))
	for _, pg := range dvpgs {
		dvpgByMoid[pg.Key] = pg
	}

	out := make([]SwitchInfo, 0, len(hosts)+len(dvss))

	for _, h := range hosts {
		out = append(out, standardSwitches(h)...)
	}
	for _, dvs := range dvss {
		sw, err := distributedSwitch(ctx, c, dvs, dvpgByMoid)
		if err != nil {
			return nil, err
		}
		out = append(out, sw)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}

// standardSwitches maps one host's network config to SwitchInfo values,
// one per virtual switch on the host.
func standardSwitches(h mo.HostSystem) []SwitchInfo {
	net := h.Config.Network
	if net == nil || len(net.Vswitch) == 0 {
		return nil
	}

	// Group port groups by their owning switch name.
	pgsBySwitch := make(map[string][]PortGroupInfo)
	for _, pg := range net.Portgroup {
		pgsBySwitch[pg.Spec.VswitchName] = append(pgsBySwitch[pg.Spec.VswitchName], PortGroupInfo{
			Name: pg.Spec.Name,
			VLAN: FormatStandardVLAN(pg.Spec.VlanId),
		})
	}

	switches := make([]SwitchInfo, 0, len(net.Vswitch))
	for _, vs := range net.Vswitch {
		pgs := pgsBySwitch[vs.Name]
		sort.Slice(pgs, func(i, j int) bool { return pgs[i].Name < pgs[j].Name })
		total := int64(vs.NumPorts)
		used := total - int64(vs.NumPortsAvailable)
		if used < 0 {
			used = 0
		}
		switches = append(switches, SwitchInfo{
			Name:       vs.Name,
			Kind:       SwitchStandard,
			Uplinks:    uplinkDevices(vs),
			LACP:       LACPNotApplicable,
			TotalPorts: total,
			UsedPorts:  used,
			PortGroups: pgs,
		})
	}
	return switches
}

// uplinkDevices lists the physical NICs backing a standard switch bridge.
func uplinkDevices(vs types.HostVirtualSwitch) []string {
	switch bridge := vs.Spec.Bridge.(type) {
	case *types.HostVirtualSwitchBondBridge:
		return bridge.NicDevice
	case *types.HostVirtualSwitchSimpleBridge:
		return []string{bridge.NicDevice}
	default:
		return nil
	}
}

// distributedSwitch maps a DVS and its port groups to a SwitchInfo.
func distributedSwitch(
	ctx context.Context,
	c *govmomi.Client,
	dvs mo.DistributedVirtualSwitch,
	dvpgByMoid map[string]mo.DistributedVirtualPortgroup,
) (SwitchInfo, error) {
	sw := SwitchInfo{
		Name: dvs.Name,
		Kind: SwitchDistributed,
	}

	cfg := dvs.Config.GetDVSConfigInfo()
	sw.TotalPorts = int64(cfg.NumPorts)

	// Uplink port groups name the uplink ports of the switch.
	var uplinks []string
	for _, ref := range cfg.UplinkPortgroup {
		if pg, ok := dvpgByMoid[ref.Value]; ok {
			uplinks = append(uplinks, pg.Name)
		} else {
			uplinks = append(uplinks, ref.Value)
		}
	}
	sw.Uplinks = uplinks

	// LACP applies to distributed switches only: it is enabled when the
	// switch has LACP groups configured.
	sw.LACP = LACPDisabled
	if vmware, ok := dvs.Config.(*types.VMwareDVSConfigInfo); ok && len(vmware.LacpGroupConfig) > 0 {
		sw.LACP = LACPEnabled
	}

	// Port groups of the switch: the DVS entity links them directly.
	pgs := make([]PortGroupInfo, 0, len(dvs.Portgroup))
	for _, ref := range dvs.Portgroup {
		pg, ok := dvpgByMoid[ref.Value]
		if !ok {
			continue
		}
		pgs = append(pgs, PortGroupInfo{
			Name: pg.Name,
			VLAN: FormatDistributedVLAN(pg.Config.DefaultPortConfig),
		})
	}
	sort.Slice(pgs, func(i, j int) bool { return pgs[i].Name < pgs[j].Name })
	sw.PortGroups = pgs

	// Count existing ports and how many are in use (have a connectee).
	ports, err := object.NewDistributedVirtualSwitch(c.Client, dvs.Reference()).FetchDVPorts(ctx, nil)
	if err != nil {
		// Without port data we cannot report usage truthfully.
		return SwitchInfo{}, fmt.Errorf("fetch ports of distributed switch %s: %w", dvs.Name, err)
	}
	if sw.TotalPorts == 0 {
		sw.TotalPorts = int64(len(ports))
	}
	var used int64
	for _, p := range ports {
		if p.Connectee != nil {
			used++
		}
	}
	sw.UsedPorts = used

	return sw, nil
}

// ListPortGroupVMs returns the virtual machines attached to the named port
// group, which may be a standard (host) port group or a distributed one.
// Sorted by VM name.
func ListPortGroupVMs(ctx context.Context, c *govmomi.Client, portGroupName string) ([]VMInfo, error) {
	v, err := newContainerView(ctx, c, []string{"HostSystem", "DistributedVirtualPortgroup", "VirtualMachine"})
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "config.network"}, &hosts)
	if err != nil {
		return nil, fmt.Errorf("retrieve hosts: %w", err)
	}

	var dvpgs []mo.DistributedVirtualPortgroup
	err = v.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "key"}, &dvpgs)
	if err != nil {
		return nil, fmt.Errorf("retrieve distributed port groups: %w", err)
	}

	stdExists := false
	for _, h := range hosts {
		if h.Config.Network == nil {
			continue
		}
		for _, pg := range h.Config.Network.Portgroup {
			if pg.Spec.Name == portGroupName {
				stdExists = true
			}
		}
	}

	var dvpgKeys []string
	dvExists := false
	for _, pg := range dvpgs {
		if pg.Name == portGroupName {
			dvExists = true
			dvpgKeys = append(dvpgKeys, pg.Key)
		}
	}

	if !stdExists && !dvExists {
		return nil, fmt.Errorf("port group %q not found in the inventory (neither standard nor distributed)", portGroupName)
	}

	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{
		"name",
		"summary.config",
		"summary.storage",
		"config.hardware.device",
	}, &vms)
	if err != nil {
		return nil, fmt.Errorf("retrieve virtual machines: %w", err)
	}

	out := make([]VMInfo, 0)
	for _, vm := range vms {
		if vmConnectedTo(vm, portGroupName, dvpgKeys) {
			info := VMInfo{Name: vm.Name}
			if s := vm.Summary.Config; s.Name != "" || s.NumCpu > 0 || s.MemorySizeMB > 0 {
				if s.Name != "" {
					info.Name = s.Name
				}
				info.NumCPU = s.NumCpu
				info.MemoryMB = s.MemorySizeMB
			}
			if s := vm.Summary.Storage; s != nil {
				info.StorageCommittedBytes = s.Committed
			}
			out = append(out, info)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// vmConnectedTo reports whether any of the VM's ethernet cards is backed by
// the named standard port group or by one of the given distributed port
// group keys.
func vmConnectedTo(vm mo.VirtualMachine, portGroupName string, dvpgKeys []string) bool {
	if vm.Config == nil {
		return false
	}
	keySet := make(map[string]bool, len(dvpgKeys))
	for _, k := range dvpgKeys {
		keySet[k] = true
	}
	for _, d := range vm.Config.Hardware.Device {
		nic, ok := d.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		switch backing := nic.GetVirtualEthernetCard().Backing.(type) {
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if backing.DeviceName == portGroupName {
				return true
			}
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			if keySet[backing.Port.PortgroupKey] {
				return true
			}
		case *types.VirtualEthernetCardOpaqueNetworkBackingInfo:
			if backing.OpaqueNetworkId == portGroupName {
				return true
			}
		}
	}
	return false
}

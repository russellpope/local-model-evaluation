package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// VMsOnPortgroup returns the virtual machines connected to the named port
// group. It works for both standard portgroups (matched by portgroup name or
// host key) and distributed portgroups (matched by dvportgroup key).
func VMsOnPortgroup(ctx context.Context, c *vim25.Client, name string) ([]VMInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("portgroup name must not be empty")
	}

	backingIDs := make(map[string]bool)

	// Distributed portgroups: the VM NIC backing references the portgroup key.
	dvpgRefs, err := listKind(ctx, c, "DistributedVirtualPortgroup")
	if err != nil {
		return nil, fmt.Errorf("enumerate distributed portgroups: %w", err)
	}
	var dvpgs []mo.DistributedVirtualPortgroup
	if err := collect(ctx, c, dvpgRefs, []string{"name", "key", "config"}, &dvpgs); err != nil {
		return nil, fmt.Errorf("list distributed portgroups: %w", err)
	}
	for i := range dvpgs {
		if pg := &dvpgs[i]; pg.Config.Name == name || pg.Name == name {
			backingIDs[pg.Config.Key] = true
			backingIDs[pg.Self.Value] = true
		}
	}

	// Standard portgroups: the VM NIC backing references the portgroup name
	// (some servers report the host-scoped portgroup key instead).
	hostRefs, err := listKind(ctx, c, "HostSystem")
	if err != nil {
		return nil, err
	}
	var hosts []mo.HostSystem
	if err := collect(ctx, c, hostRefs, []string{"config"}, &hosts); err != nil {
		return nil, fmt.Errorf("list host networks: %w", err)
	}
	for i := range hosts {
		h := &hosts[i]
		if h.Config == nil || h.Config.Network == nil {
			continue
		}
		for _, pg := range h.Config.Network.Portgroup {
			if pg.Spec.Name == name {
				backingIDs[pg.Spec.Name] = true
				backingIDs[pg.Key] = true
			}
		}
	}

	if len(backingIDs) == 0 {
		return nil, fmt.Errorf("portgroup %q not found in the inventory", name)
	}

	vmRefs, err := listKind(ctx, c, "VirtualMachine")
	if err != nil {
		return nil, err
	}
	var vms []mo.VirtualMachine
	if err := collect(ctx, c, vmRefs, []string{"name", "summary", "config.hardware.device"}, &vms); err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	var out []VMInfo
	for i := range vms {
		vm := &vms[i]
		matched := false
		for _, key := range connectedPortgroupKeys(vm) {
			if backingIDs[key] {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		info := VMInfo{Name: vm.Name}
		info.VCPU = vm.Summary.Config.NumCpu
		info.MemoryMB = vm.Summary.Config.MemorySizeMB
		if vm.Summary.Storage != nil {
			info.Storage = vm.Summary.Storage.Committed
			info.StorageKnown = true
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

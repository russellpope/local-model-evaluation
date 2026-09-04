package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMsOnPortgroup returns the virtual machines with a network adapter
// attached to the named port group — standard or distributed. Standard
// matching compares EthernetCard backing DeviceName; distributed matching
// resolves the port group's key and compares portgroupKey backings.
func VMsOnPortgroup(ctx context.Context, c *vim25.Client, name string) ([]VMInfo, error) {
	var nets []mo.Network
	_ = retrieveAll(ctx, c, "Network", []string{"name"}, &nets)

	// Resolve distributed port groups by name.
	var dvpgs []mo.DistributedVirtualPortgroup
	if err := retrieveAll(ctx, c, "DistributedVirtualPortgroup",
		[]string{"name", "key", "config", "vm"}, &dvpgs); err != nil {
		return nil, fmt.Errorf("list distributed portgroups: %w", err)
	}

	isStandard := false
	isDistributed := false
	var keys []string
	for _, pg := range dvpgs {
		if pg.Name == name {
			isDistributed = true
			keys = append(keys, pg.Key)
		}
	}
	for _, n := range nets {
		if n.Name == name {
			isStandard = true
		}
	}
	if !isStandard && !isDistributed {
		return nil, fmt.Errorf("port group %q not found", name)
	}

	var vms []mo.VirtualMachine
	if err := retrieveAll(ctx, c, "VirtualMachine",
		[]string{"name", "config.hardware", "summary.storage"}, &vms); err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}

	out := []VMInfo{}
	for i := range vms {
		if !vmOnPortgroup(&vms[i], name, keySet) {
			continue
		}
		vm := vms[i]
		info := VMInfo{Name: vm.Name}
		if vm.Config != nil {
			info.NumCPU = vm.Config.Hardware.NumCPU
			info.MemoryMB = int64(vm.Config.Hardware.MemoryMB)
		}
		if vm.Summary.Storage != nil {
			info.Committed = vm.Summary.Storage.Committed
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func vmOnPortgroup(vm *mo.VirtualMachine, pgName string, dvpgKeys map[string]bool) bool {
	if vm.Config == nil {
		return false
	}
	for _, dev := range vm.Config.Hardware.Device {
		nic, ok := dev.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		switch b := nic.GetVirtualEthernetCard().Backing.(type) {
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if b.DeviceName == pgName {
				return true
			}
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			if dvpgKeys[b.Port.PortgroupKey] {
				return true
			}
		}
	}
	return false
}

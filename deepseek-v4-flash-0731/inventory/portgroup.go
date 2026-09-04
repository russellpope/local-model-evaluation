package inventory

import (
	"context"
	"fmt"
	"path"
	"sort"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// nameOf returns an entity's display name from its inventory path. Finder
// results always carry a path (e.g. "/DC0/network/DC0_DVPG0").
func nameOf(n object.NetworkReference) string {
	return path.Base(n.GetInventoryPath())
}

// VMsOnPortGroup returns the VMs whose virtual NICs are connected to the named
// port group. Both standard host port groups (Network entities) and
// distributed port groups (DistributedVirtualPortgroup) are supported. The
// result is sorted by VM name.
func VMsOnPortGroup(ctx context.Context, c *vim25.Client, portgroup string) ([]VMInfo, error) {
	var target types.ManagedObjectReference
	found := false
	err := withDatacenters(ctx, c, func(f *find.Finder) error {
		nets, err := f.NetworkList(ctx, "*")
		if err != nil {
			return err
		}
		for _, n := range nets {
			if !found && nameOf(n) == portgroup {
				target = n.Reference()
				found = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find networks: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("port group %q not found", portgroup)
	}

	var refs []types.ManagedObjectReference
	err = withDatacenters(ctx, c, func(f *find.Finder) error {
		vms, err := f.VirtualMachineList(ctx, "*")
		if err != nil {
			return err
		}
		for _, vm := range vms {
			refs = append(refs, vm.Reference())
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find virtual machines: %w", err)
	}
	refs = dedupeRefs(refs)

	var mes []mo.VirtualMachine
	err = property.DefaultCollector(c).Retrieve(ctx, refs,
		[]string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "network"}, &mes)
	if err != nil {
		return nil, fmt.Errorf("retrieve virtual machine properties: %w", err)
	}

	var out []VMInfo
	for i := range mes {
		vm := &mes[i]
		if !containsRef(vm.Network, target) {
			continue
		}
		info := VMInfo{Name: vm.Name}
		if vm.Config != nil {
			info.CPUs = vm.Config.Hardware.NumCPU
			info.RAMMB = int64(vm.Config.Hardware.MemoryMB)
		}
		out = append(out, info)
	}
	sortVMs(out)
	return out, nil
}

func containsRef(refs []types.ManagedObjectReference, want types.ManagedObjectReference) bool {
	for _, r := range refs {
		if r == want {
			return true
		}
	}
	return false
}

// sortVMs orders VMs by name (ascending), stable and in place.
func sortVMs(vms []VMInfo) {
	sort.SliceStable(vms, func(i, j int) bool { return vms[i].Name < vms[j].Name })
}

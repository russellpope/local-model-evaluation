package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// VMsInPortgroup returns the virtual machines that have a network device
// connected to the named port group. It works for both standard (host
// vSwitch) and distributed (vDS) port groups, because a VM's network
// property lists every port group its NICs are attached to, regardless of
// the underlying switch type.
// networkViewTypes are every network kind a port group can be. They are
// listed explicitly because a container view does not expand subtypes.
var networkViewTypes = []string{
	"Network",
	"DistributedVirtualPortgroup",
	"OpaqueNetwork",
}

func VMsInPortgroup(ctx context.Context, c *vim25.Client, name string) ([]VMInfo, error) {
	refs, err := viewRefs(ctx, c, networkViewTypes)
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	match := map[string]bool{}
	if len(refs) > 0 {
		var objs []mo.Network
		if err := property.DefaultCollector(c).Retrieve(ctx, refs,
			[]string{"name"}, &objs); err != nil {
			return nil, fmt.Errorf("collect network names: %w", err)
		}
		seen := map[string]bool{}
		for i := range objs {
			if seen[objs[i].Self.Value] {
				continue
			}
			seen[objs[i].Self.Value] = true
			if objs[i].Name == name {
				match[objs[i].Self.Value] = true
			}
		}
	}

	if len(match) == 0 {
		return nil, fmt.Errorf("no port group or network named %q found in the inventory", name)
	}

	vmRefs, err := findVMRefs(ctx, c)
	if err != nil {
		return nil, err
	}
	if len(vmRefs) == 0 {
		return nil, nil
	}

	objs, err := collectVMs(ctx, c, vmRefs)
	if err != nil {
		return nil, err
	}

	infos := make([]VMInfo, 0)
	for i := range objs {
		vm := &objs[i]
		if vm.Config == nil {
			continue
		}
		if !vmInPortgroup(vm, match) {
			continue
		}
		infos = append(infos, vmInfo(vm))
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

func vmInPortgroup(vm *mo.VirtualMachine, match map[string]bool) bool {
	for _, ref := range vm.Network {
		if match[ref.Value] {
			return true
		}
	}
	return false
}

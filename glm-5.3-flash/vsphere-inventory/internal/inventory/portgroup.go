package inventory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMsOnPortgroup returns the VMs connected to the named port group, sorted
// by name. The lookup works for standard port groups (matched by network
// name or vNIC device name) and distributed port groups (matched by name,
// port group key, or the VM's network references).
func VMsOnPortgroup(ctx context.Context, c *vim25.Client, name string) ([]VMInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New(`port group name is empty: pass --portgroup "<name>"`)
	}

	refs := map[types.ManagedObjectReference]struct{}{}
	dvKeys := map[string]struct{}{}

	networks, err := findNetworksByName(ctx, c, name)
	if err != nil {
		return nil, err
	}
	dvpgs, err := findDVPortgroupsByName(ctx, c, name)
	if err != nil {
		return nil, err
	}
	for _, ref := range networks {
		refs[ref] = struct{}{}
	}
	for _, pg := range dvpgs {
		refs[pg.Self] = struct{}{}
		if pg.Key != "" {
			dvKeys[pg.Key] = struct{}{}
		}
	}
	if len(refs) == 0 {
		return nil, fmt.Errorf("port group %q not found in inventory", name)
	}

	v, err := view.NewManager(c).CreateContainerView(
		ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for virtual machines: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"},
		[]string{"name", "config.hardware.device", "network"}, &vms)
	if err != nil {
		return nil, fmt.Errorf("retrieve virtual machines: %w", err)
	}

	out := make([]VMInfo, 0, len(vms))
	for i := range vms {
		vm := &vms[i]
		if vmUsesPortgroup(vm, name, refs, dvKeys) {
			out = append(out, VMInfo{Name: vmDisplayName(vm)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func findNetworksByName(ctx context.Context, c *vim25.Client, name string) ([]types.ManagedObjectReference, error) {
	v, err := view.NewManager(c).CreateContainerView(
		ctx, c.ServiceContent.RootFolder, []string{"Network"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for networks: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var nets []mo.Network
	if err := v.Retrieve(ctx, []string{"Network"}, []string{"name"}, &nets); err != nil {
		return nil, fmt.Errorf("retrieve networks: %w", err)
	}
	var refs []types.ManagedObjectReference
	for i := range nets {
		if nets[i].Name == name {
			refs = append(refs, nets[i].Self)
		}
	}
	return refs, nil
}

func findDVPortgroupsByName(ctx context.Context, c *vim25.Client, name string) ([]mo.DistributedVirtualPortgroup, error) {
	v, err := view.NewManager(c).CreateContainerView(
		ctx, c.ServiceContent.RootFolder, []string{"DistributedVirtualPortgroup"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for port groups: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var pgs []mo.DistributedVirtualPortgroup
	if err := v.Retrieve(ctx, []string{"DistributedVirtualPortgroup"}, []string{"name", "key"}, &pgs); err != nil {
		return nil, fmt.Errorf("retrieve distributed port groups: %w", err)
	}
	var matched []mo.DistributedVirtualPortgroup
	for i := range pgs {
		if pgs[i].Name == name || pgs[i].Key == name {
			matched = append(matched, pgs[i])
		}
	}
	return matched, nil
}

// vmUsesPortgroup matches a VM against a port group through its vNIC
// backings and its network references.
func vmUsesPortgroup(vm *mo.VirtualMachine, name string, refs map[types.ManagedObjectReference]struct{}, dvKeys map[string]struct{}) bool {
	for _, ref := range vm.Network {
		if _, ok := refs[ref]; ok {
			return true
		}
	}
	if vm.Config == nil {
		return false
	}
	for _, dev := range vm.Config.Hardware.Device {
		nic, ok := dev.(types.BaseVirtualEthernetCard)
		if !ok {
			continue
		}
		switch backing := nic.GetVirtualEthernetCard().Backing.(type) {
		case *types.VirtualEthernetCardNetworkBackingInfo:
			if backing != nil && backing.DeviceName == name {
				return true
			}
		case *types.VirtualEthernetCardDistributedVirtualPortBackingInfo:
			if backing == nil {
				continue
			}
			if backing.Port.PortgroupKey == name {
				return true
			}
			if _, ok := dvKeys[backing.Port.PortgroupKey]; ok {
				return true
			}
		}
	}
	return false
}

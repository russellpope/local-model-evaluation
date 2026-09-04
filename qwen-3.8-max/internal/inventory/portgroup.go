package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func VMsOnPortgroup(ctx context.Context, c *vim25.Client, portgroup string) ([]VMInfo, error) {
	refs, exists, err := portgroupNetworkRefs(ctx, c, portgroup)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("port group %q not found (searched standard and distributed port groups)", portgroup)
	}
	wanted := make(map[types.ManagedObjectReference]struct{}, len(refs))
	for _, ref := range refs {
		wanted[ref] = struct{}{}
	}

	vms, err := retrieveVMs(ctx, c, []string{"name", "network"})
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	infos := []VMInfo{}
	for _, vm := range vms {
		for _, net := range vm.Network {
			if _, ok := wanted[net]; ok {
				infos = append(infos, VMInfo{Name: vm.Name})
				break
			}
		}
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

func portgroupNetworkRefs(ctx context.Context, c *vim25.Client, portgroup string) ([]types.ManagedObjectReference, bool, error) {
	var refs []types.ManagedObjectReference
	exists := false

	var networks []mo.Network
	if err := containerRetrieve(ctx, c, []string{"Network"}, []string{"name"}, &networks); err != nil {
		return nil, false, fmt.Errorf("search standard port groups: %w", err)
	}
	for _, net := range networks {
		if net.Name == portgroup {
			exists = true
			refs = append(refs, net.Self)
		}
	}

	var portgroups []mo.DistributedVirtualPortgroup
	if err := containerRetrieve(ctx, c, []string{"DistributedVirtualPortgroup"}, []string{"config.name"}, &portgroups); err != nil {
		return nil, false, fmt.Errorf("search distributed port groups: %w", err)
	}
	for _, pg := range portgroups {
		if pg.Config.Name == portgroup {
			exists = true
			refs = append(refs, pg.Self)
		}
	}

	if !exists {
		var hosts []mo.HostSystem
		if err := containerRetrieve(ctx, c, []string{"HostSystem"}, []string{"config.network"}, &hosts); err != nil {
			return nil, false, fmt.Errorf("search host port groups: %w", err)
		}
		for _, host := range hosts {
			if host.Config == nil || host.Config.Network == nil {
				continue
			}
			for _, pg := range host.Config.Network.Portgroup {
				if pg.Spec.Name == portgroup {
					exists = true
					break
				}
			}
			if exists {
				break
			}
		}
	}
	return refs, exists, nil
}

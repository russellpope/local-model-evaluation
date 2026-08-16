package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMsInPortgroup returns the virtual machines connected to the named port
// group, sorted by name. It works for both standard vswitch port groups
// and distributed virtual port groups: both are network objects, and a VM
// is connected to a port group when one of its network devices references
// the port group's network object.
func VMsInPortgroup(ctx context.Context, c *vim25.Client, name string) ([]VMInfo, error) {
	if name == "" {
		return nil, fmt.Errorf("port group name is empty")
	}

	refs, err := findRefs(ctx, c, "VirtualMachine")
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	var content []mo.VirtualMachine
	if err := retrieve(ctx, c, refs, []string{"name", "summary", "network"}, &content); err != nil {
		return nil, err
	}

	netSeen := make(map[string]bool)
	var netRefs []types.ManagedObjectReference
	for _, m := range content {
		for _, n := range m.Network {
			if netSeen[n.Value] {
				continue
			}
			netSeen[n.Value] = true
			netRefs = append(netRefs, n)
		}
	}

	// Resolve the network objects the VMs actually reference and match
	// them by name against the requested port group.
	names, err := retrieveRaw(ctx, c, netRefs, "Network", []string{"name"})
	if err != nil {
		return nil, err
	}

	wanted := make(map[string]bool)
	for _, ref := range netRefs {
		props, ok := names[ref.Value]
		if !ok {
			continue
		}
		if n, _ := props["name"].(string); n == name {
			wanted[ref.Value] = true
		}
	}
	if len(wanted) == 0 {
		return nil, fmt.Errorf("port group %q not found in inventory", name)
	}

	out := make([]VMInfo, 0)
	for _, m := range content {
		connected := false
		for _, n := range m.Network {
			if wanted[n.Value] {
				connected = true
				break
			}
		}
		if !connected {
			continue
		}
		info := VMInfo{
			Name:     m.Name,
			VCPU:     m.Summary.Config.NumCpu,
			MemoryMB: m.Summary.Config.MemorySizeMB,
		}
		if m.Summary.Storage != nil {
			info.Committed = m.Summary.Storage.Committed
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

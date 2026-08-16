package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMInfo is one row of the vms report.
type VMInfo struct {
	Name string
	VCPU int32
	// MemoryMB is the configured memory in MiB.
	MemoryMB int32
	// Committed is the storage actually consumed (bytes), as reported by
	// the server summary. This is committed usage, not provisioned or
	// allocated capacity.
	Committed int64
}

// ListVMs returns every virtual machine in the inventory, sorted by name.
func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	refs, err := findRefs(ctx, c, "VirtualMachine")
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	return fetchVMs(ctx, c, refs)
}

func fetchVMs(ctx context.Context, c *vim25.Client, refs []types.ManagedObjectReference) ([]VMInfo, error) {
	var content []mo.VirtualMachine
	if err := retrieve(ctx, c, refs, []string{"name", "summary"}, &content); err != nil {
		return nil, err
	}

	out := make([]VMInfo, 0, len(content))
	for _, m := range content {
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

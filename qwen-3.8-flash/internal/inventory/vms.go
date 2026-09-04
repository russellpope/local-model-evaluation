package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// VMInfo is one row of the vms table.
type VMInfo struct {
	Name     string
	VCPU     int32
	MemoryMB int32
	// Storage is actual storage consumed (committed) in bytes.
	Storage      int64
	StorageKnown bool
}

// ListVMs returns every virtual machine's configured compute and committed
// storage, sorted by name.
func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	refs, err := listKind(ctx, c, "VirtualMachine")
	if err != nil {
		return nil, err
	}
	var machines []mo.VirtualMachine
	if err := collect(ctx, c, refs, []string{"name", "summary"}, &machines); err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	out := make([]VMInfo, 0, len(machines))
	for i := range machines {
		vm := &machines[i]
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

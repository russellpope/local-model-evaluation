package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/mo"
)

// VMInfo describes a virtual machine. StorageCommittedBytes is the storage
// actually consumed (committed) by the VM, not the provisioned capacity.
type VMInfo struct {
	Name                  string
	NumCPU                int32
	MemoryMB              int32
	StorageCommittedBytes int64
}

// ListVMs returns all virtual machines in the inventory, sorted by name.
func ListVMs(ctx context.Context, c *govmomi.Client) ([]VMInfo, error) {
	v, err := newContainerView(ctx, c, []string{"VirtualMachine"})
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{
		"name",
		"summary.config",
		"summary.storage",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
	}, &vms)
	if err != nil {
		return nil, fmt.Errorf("retrieve virtual machines: %w", err)
	}

	out := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		info := VMInfo{Name: vm.Name}
		if cfg := vm.Config; cfg != nil {
			info.NumCPU = cfg.Hardware.NumCPU
			info.MemoryMB = cfg.Hardware.MemoryMB
		}
		if s := vm.Summary.Config; s.Name != "" || s.NumCpu > 0 || s.MemorySizeMB > 0 {
			if s.Name != "" {
				info.Name = s.Name
			}
			if s.NumCpu > 0 {
				info.NumCPU = s.NumCpu
			}
			if s.MemorySizeMB > 0 {
				info.MemoryMB = s.MemorySizeMB
			}
		}
		if s := vm.Summary.Storage; s != nil {
			info.StorageCommittedBytes = s.Committed
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

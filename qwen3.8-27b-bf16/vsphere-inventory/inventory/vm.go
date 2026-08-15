package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// StorageUnknown is the StorageBytes value used when the API does not report
// how much storage a VM has committed.
const StorageUnknown int64 = -1

// VMInfo is one row of the vms table.
type VMInfo struct {
	Name         string
	VCPU         int
	RAMBytes     int64
	StorageBytes int64
}

func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
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

	infos := make([]VMInfo, 0, len(objs))
	for i := range objs {
		infos = append(infos, vmInfo(&objs[i]))
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// vmInfo converts a collected mo.VirtualMachine into a VMInfo row.
func vmInfo(vm *mo.VirtualMachine) VMInfo {
	info := VMInfo{Name: vm.Name, StorageBytes: committedBytes(vm)}
	if vm.Config != nil {
		info.VCPU = int(vm.Config.Hardware.NumCPU)
		info.RAMBytes = int64(vm.Config.Hardware.MemoryMB) * mib
	}
	return info
}

func findVMRefs(ctx context.Context, c *vim25.Client) ([]types.ManagedObjectReference, error) {
	refs, err := viewRefs(ctx, c, []string{"VirtualMachine"})
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}
	return refs, nil
}

func collectVMs(ctx context.Context, c *vim25.Client, refs []types.ManagedObjectReference) ([]mo.VirtualMachine, error) {
	props := []string{
		"name",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
		"summary.storage.committed",
		"layoutEx.file",
		"network",
	}
	var objs []mo.VirtualMachine
	if err := property.DefaultCollector(c).Retrieve(ctx, refs, props, &objs); err != nil {
		return nil, fmt.Errorf("collect virtual machine properties: %w", err)
	}
	return objs, nil
}

// committedBytes returns the storage a VM has actually consumed (committed),
// not the provisioned/allocated capacity. It prefers the vCenter-reported
// committed figure and falls back to summing the on-disk size of every file
// in the VM's layout.
func committedBytes(vm *mo.VirtualMachine) int64 {
	if vm.Summary.Storage != nil && vm.Summary.Storage.Committed > 0 {
		return vm.Summary.Storage.Committed
	}
	if vm.LayoutEx != nil {
		var total int64
		for _, f := range vm.LayoutEx.File {
			total += f.Size
		}
		if total > 0 {
			return total
		}
	}
	return StorageUnknown
}

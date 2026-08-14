package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
)

func ListVMs(ctx context.Context, c *object.Datacenter) ([]VMInfo, error) {
	vmgr := view.NewManager(c.Client())
	v, err := vmgr.CreateContainerView(ctx, c.Reference(), []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create VM view: %w", err)
	}
	defer v.Destroy(ctx)

	var vmList []mo.VirtualMachine
	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "storage.perDatastoreUsage"}, &vmList); err != nil {
		return nil, fmt.Errorf("retrieve VMs: %w", err)
	}

	var result []VMInfo
	for _, vm := range vmList {
		vcpu := vm.Config.Hardware.NumCPU
		if vcpu == 0 {
			vcpu = 1
		}
		ramGB := float64(vm.Config.Hardware.MemoryMB) / 1024.0

		var storageGiB float64
		if vm.Storage != nil {
			for _, usage := range vm.Storage.PerDatastoreUsage {
				storageGiB += float64(usage.Committed) / (1024 * 1024 * 1024)
			}
		}

		result = append(result, VMInfo{
			Name:       vm.Name,
			VCPU:       vcpu,
			RAMGB:      ramGB,
			StorageGiB: storageGiB,
		})
	}

	sortByName(result)
	return result, nil
}

func sortByName(vms []VMInfo) {
	for i := 1; i < len(vms); i++ {
		for j := i; j > 0 && vms[j].Name < vms[j-1].Name; j-- {
			vms[j], vms[j-1] = vms[j-1], vms[j]
		}
	}
}

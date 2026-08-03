package vms

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

type VMInfo struct {
	Name         string
	VCPU         int
	RAMMB        int
	StorageBytes int64
}

func GetVMs(ctx context.Context, client *vim25.Client) ([]VMInfo, error) {
	finder := find.NewFinder(client)
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("finding default datacenter: %w", err)
	}
	finder.SetDatacenter(dc)

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing virtual machines: %w", err)
	}

	var result []VMInfo
	for _, vm := range vms {
		var vmMo mo.VirtualMachine
		err := vm.Properties(ctx, vm.Reference(), []string{
			"name",
			"config.hardware.numCPU",
			"config.hardware.memoryMB",
			"summary.storage.committed",
		}, &vmMo)
		if err != nil {
			return nil, fmt.Errorf("retrieving properties for VM %s: %w", vm.Name(), err)
		}

		var committed int64
		if vmMo.Summary.Storage != nil {
			committed = vmMo.Summary.Storage.Committed
		}

		result = append(result, VMInfo{
			Name:         vmMo.Name,
			VCPU:         int(vmMo.Config.Hardware.NumCPU),
			RAMMB:        int(vmMo.Config.Hardware.MemoryMB),
			StorageBytes: committed,
		})
	}

	return result, nil
}

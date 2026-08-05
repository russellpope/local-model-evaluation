package vms

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

type VMInfo struct {
	Name    string
	VCPU    int32
	RAM     int32
	Storage int64
}

func GetVMs(ctx context.Context, client *vim25.Client) ([]VMInfo, error) {
	finder := find.NewFinder(client)

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	var results []VMInfo

	for _, vm := range vms {
		info, err := getVMInfo(ctx, vm)
		if err != nil {
			return nil, fmt.Errorf("get VM info for %q: %w", vm.Name(), err)
		}
		results = append(results, info)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func getVMInfo(ctx context.Context, vm *object.VirtualMachine) (VMInfo, error) {
	var vmMo mo.VirtualMachine

	props := []string{
		"name",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
		"summary.storage.committed",
	}

	if err := vm.Properties(ctx, vm.Reference(), props, &vmMo); err != nil {
		return VMInfo{}, fmt.Errorf("retrieve VM properties: %w", err)
	}

	info := VMInfo{
		Name: vmMo.Name,
	}

	if vmMo.Config != nil {
		info.VCPU = vmMo.Config.Hardware.NumCPU
		info.RAM = vmMo.Config.Hardware.MemoryMB
	}

	if vmMo.Summary.Storage != nil {
		info.Storage = vmMo.Summary.Storage.Committed
	}

	return info, nil
}

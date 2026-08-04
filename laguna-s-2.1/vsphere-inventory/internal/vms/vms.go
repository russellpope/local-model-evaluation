package vms

import (
	"context"
	"fmt"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/model"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

func GetVMs(ctx context.Context, client *vim25.Client) ([]model.VMInfo, error) {
	finder := find.NewFinder(client)

	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datacenters: %w", err)
	}

	var result []model.VMInfo
	for _, dc := range dcs {
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing virtual machines in datacenter %s: %w", dc.Name(), err)
		}

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

			var vcpu int
			var ramMB int
			if vmMo.Config != nil {
				vcpu = int(vmMo.Config.Hardware.NumCPU)
				ramMB = int(vmMo.Config.Hardware.MemoryMB)
			}

			result = append(result, model.VMInfo{
				Name:         vmMo.Name,
				VCPU:         vcpu,
				RAMMB:        ramMB,
				StorageBytes: committed,
			})
		}
	}

	return result, nil
}

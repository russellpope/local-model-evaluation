package vms

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/model"
)

func ListVMs(ctx context.Context, client *vim25.Client) ([]model.VMInfo, error) {
	finder := find.NewFinder(client, false)
	
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default datacenter: %w", err)
	}
	
	finder.SetDatacenter(dc)
	
	vmList, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %w", err)
	}
	
	var vms []model.VMInfo
	for _, vm := range vmList {
		var props struct {
			Name      string
			Config    types.VirtualMachineConfigInfo
			Storage   types.VirtualMachineStorageInfo
		}
		
		pc := property.DefaultCollector(client)
		if err := pc.RetrieveOne(ctx, vm.Reference(), []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "storage"}, &props); err != nil {
			return nil, fmt.Errorf("failed to get VM properties: %w", err)
		}
		
		var totalStorage int64
		for _, usage := range props.Storage.PerDatastoreUsage {
			totalStorage += usage.Committed
		}
		
		vms = append(vms, model.VMInfo{
			Name:    props.Name,
			VCPU:    props.Config.Hardware.NumCPU,
			RAM:     int64(props.Config.Hardware.MemoryMB) * 1024 * 1024,
			Storage: totalStorage,
		})
	}
	
	return vms, nil
}

func init() {
	_ = find.NewFinder
	_ = property.DefaultCollector
}

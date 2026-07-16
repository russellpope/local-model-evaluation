package storage

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

type VMInfo struct {
	Name      string
	VCPU      int32
	RAMGB     float64
	StorageGB float64
}

func GetVMs(ctx context.Context, client *govmomi.Client) ([]VMInfo, error) {
	finder := find.NewFinder(client.Client, false)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("finding datacenters: %w", err)
	}

	var allVMs []VMInfo

	for _, dc := range datacenters {
		finder.SetDatacenter(dc)
		vms, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing VMs in datacenter %s: %w", dc.Name(), err)
		}

		for _, vm := range vms {
			info, err := getVMInfo(ctx, vm)
			if err != nil {
				continue
			}
			allVMs = append(allVMs, info)
		}
	}

	sort.Slice(allVMs, func(i, j int) bool {
		return allVMs[i].Name < allVMs[j].Name
	})

	return allVMs, nil
}

func getVMInfo(ctx context.Context, vm *object.VirtualMachine) (VMInfo, error) {
	// Get the VM properties using the Properties method
	var vmMo mo.VirtualMachine
	err := vm.Properties(ctx, vm.Reference(), []string{"name", "summary.config", "summary.storage"}, &vmMo)
	if err != nil {
		return VMInfo{}, fmt.Errorf("getting VM properties: %w", err)
	}

	summary := vmMo.Summary

	// Check if config is available
	if vmMo.Config == nil {
		return VMInfo{Name: vmMo.Name}, nil
	}

	config := vmMo.Config

	name := vmMo.Name
	vcpu := config.Hardware.NumCPU
	ramBytes := config.Hardware.MemoryMB * 1024 * 1024

	// Calculate consumed storage
	storageBytes := int64(0)
	if summary.Storage != nil {
		storageBytes = summary.Storage.Committed
	}

	return VMInfo{
		Name:      name,
		VCPU:      vcpu,
		RAMGB:     float64(ramBytes) / (1024 * 1024 * 1024),
		StorageGB: float64(storageBytes) / (1024 * 1024 * 1024),
	}, nil
}

package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
)

func FindVMsInPortGroup(ctx context.Context, client *govmomi.Client, portGroupName string) ([]VMInfo, error) {
	fm := property.DefaultCollector(client.Client)
	finder := find.NewFinder(client.Client, false)

	dc, err := finder.DefaultDatacenter(ctx)
	if err == nil {
		finder.SetDatacenter(dc)
	}

	net, err := finder.Network(ctx, portGroupName)
	if err != nil {
		return nil, fmt.Errorf("find port group %q: %w", portGroupName, err)
	}

	var moNet mo.Network
	err = fm.RetrieveOne(ctx, net.Reference(), []string{"vm"}, &moNet)
	if err != nil {
		return nil, fmt.Errorf("retrieve network %q VMs: %w", portGroupName, err)
	}

	if len(moNet.Vm) == 0 {
		return []VMInfo{}, nil
	}

	var vms []mo.VirtualMachine
	err = fm.Retrieve(ctx, moNet.Vm, []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage.committed"}, &vms)
	if err != nil {
		return nil, fmt.Errorf("retrieve VM properties: %w", err)
	}

	var results []VMInfo
	for _, vm := range vms {
		vcpu := int32(0)
		ramMB := int32(0)
		storage := int64(0)

		if vm.Config != nil {
			vcpu = vm.Config.Hardware.NumCPU
			ramMB = vm.Config.Hardware.MemoryMB
		}
		if vm.Summary.Storage != nil {
			storage = vm.Summary.Storage.Committed
		}

		results = append(results, VMInfo{
			Name:    vm.Name,
			VCPU:    vcpu,
			RAMMB:   ramMB,
			Storage: storage,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

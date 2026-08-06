package vms

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/view"
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
	m := view.NewManager(client)

	v, err := m.CreateContainerView(ctx, client.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view: %w", err)
	}
	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine

	props := []string{
		"name",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
		"summary.storage.committed",
	}

	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, props, &vms); err != nil {
		return nil, fmt.Errorf("retrieve VMs: %w", err)
	}

	var results []VMInfo

	for _, vmMo := range vms {
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

		results = append(results, info)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

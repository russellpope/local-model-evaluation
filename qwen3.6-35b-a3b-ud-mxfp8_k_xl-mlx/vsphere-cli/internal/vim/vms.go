package vim

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/mo"
)

type VMInfo struct {
	Name    string
	VCPU    int32
	RAM     int64
	Storage int64
}

func (c *Client) GetVMs(ctx context.Context) ([]VMInfo, error) {
	v, err := c.View.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("creating VM view: %w", err)
	}
	defer v.Destroy(ctx)

	var vms []mo.VirtualMachine

	err = v.Retrieve(ctx, []string{"VirtualMachine"}, nil, &vms)
	if err != nil {
		return nil, fmt.Errorf("retrieving VMs: %w", err)
	}

	result := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		info := VMInfo{
			Name:    vm.Name,
			VCPU:    vm.Config.Hardware.NumCPU,
			RAM:     int64(vm.Config.Hardware.MemoryMB) * 1024 * 1024,
			Storage: vm.Summary.Storage.Committed,
		}
		result = append(result, info)
	}

	return result, nil
}

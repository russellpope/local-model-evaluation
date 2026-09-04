package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

type VMInfo struct {
	Name           string
	VCPU           int32
	RAMMB          int64
	CommittedBytes int64
}

func retrieveVMs(ctx context.Context, c *vim25.Client, props []string) ([]mo.VirtualMachine, error) {
	var vms []mo.VirtualMachine
	if err := containerRetrieve(ctx, c, []string{"VirtualMachine"}, props, &vms); err != nil {
		return nil, err
	}
	return vms, nil
}

func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	vms, err := retrieveVMs(ctx, c, []string{
		"name",
		"config.hardware.numCPU",
		"config.hardware.memoryMB",
		"summary.storage.committed",
	})
	if err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}
	infos := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		info := VMInfo{Name: vm.Name}
		if vm.Config != nil {
			info.VCPU = vm.Config.Hardware.NumCPU
			info.RAMMB = int64(vm.Config.Hardware.MemoryMB)
		}
		info.CommittedBytes = vm.Summary.Storage.Committed
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

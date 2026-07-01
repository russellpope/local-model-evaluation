package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"govmomi-cli/pkg/inventory/utils"
)

type VMInfo struct {
	Name    string
	VCPU    int32
	RAMGB   float64
	Storage string
}

func GetVMs(ctx context.Context, client *vim25.Client) ([]VMInfo, error) {
	view, err := getVMView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer view.Destroy(ctx)

	var vms []mo.VirtualMachine
	err = view.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage.committed"}, &vms)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve VM properties: %w", err)
	}

	var result []VMInfo
	for _, vm := range vms {
		var vcpu int32
		var ram float64
		var storage string

		if vm.Config != nil {
			vcpu = vm.Config.Hardware.NumCPU
			ram = utils.FormatRAM(vm.Config.Hardware.MemoryMB)
		}

		if vm.Summary.Storage != nil {
			storage = utils.FormatBytes(int64(vm.Summary.Storage.Committed))
		}

		result = append(result, VMInfo{
			Name:    vm.Name,
			VCPU:    vcpu,
			RAMGB:   ram,
			Storage: storage,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

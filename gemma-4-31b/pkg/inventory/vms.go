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
	Name     string
	VCPU     int32
	RAMGB    float64
	Storage  string
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
		result = append(result, VMInfo{
			Name:    vm.Name,
			VCPU:    vm.Config.Hardware.NumCPU,
			RAMGB:   utils.FormatRAM(vm.Config.Hardware.MemoryMB),
			Storage: utils.FormatBytes(int64(vm.Summary.Storage.Committed)),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

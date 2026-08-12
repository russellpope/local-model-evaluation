package inventory

import (
	"context"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/view"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/output"
)

func GetVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)
	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage.committed"}, &vms)
	if err != nil {
		return nil, err
	}
	results := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		vCPU := int32(0)
		if vm.Config != nil {
			vCPU = int32(vm.Config.Hardware.NumCPU)
		}
		ramMB := int64(0)
		if vm.Config != nil {
			ramMB = int64(vm.Config.Hardware.MemoryMB)
		}
		ramGiB := float64(ramMB) / 1024.0
		storageBytes := int64(0)
		if vm.Summary.Storage != nil {
			storageBytes = vm.Summary.Storage.Committed
		}
		results = append(results, VMInfo{
			Name:    vm.Name,
			VCPU:    vCPU,
			RAMGiB:  ramGiB,
			Storage: output.BytesToHuman(storageBytes),
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

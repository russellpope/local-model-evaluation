package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// ListVMs returns every virtual machine in the inventory, sorted by name,
// with configured vCPU/RAM and consumed (committed) storage.
func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	v, err := view.NewManager(c).CreateContainerView(
		ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for virtual machines: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var vms []mo.VirtualMachine
	err = v.Retrieve(ctx, []string{"VirtualMachine"},
		[]string{"name", "summary", "config.hardware"}, &vms)
	if err != nil {
		return nil, fmt.Errorf("retrieve virtual machines: %w", err)
	}

	out := make([]VMInfo, 0, len(vms))
	for i := range vms {
		vm := &vms[i]
		info := VMInfo{Name: vmDisplayName(vm)}
		if vm.Config != nil {
			info.VCPU = vm.Config.Hardware.NumCPU
			info.RAMMib = int64(vm.Config.Hardware.MemoryMB)
		}
		info.CommittedBytes = vm.Summary.Storage.Committed
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// vmDisplayName picks the best available name for a partially retrieved VM.
func vmDisplayName(vm *mo.VirtualMachine) string {
	if name := vm.Summary.Config.Name; name != "" {
		return name
	}
	return vm.Name
}

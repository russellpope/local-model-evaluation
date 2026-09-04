package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// VMInfo is the typed result for the vms subcommand.
type VMInfo struct {
	Name           string
	CPUs           int32
	RAMMB          int64
	CommittedBytes int64
}

// ListVMs returns every virtual machine in the inventory. CommittedBytes is
// the storage actually consumed on datastores (summary.storage.committed),
// NOT provisioned capacity.
func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	var refs []types.ManagedObjectReference
	err := withDatacenters(ctx, c, func(f *find.Finder) error {
		vms, err := f.VirtualMachineList(ctx, "*")
		if err != nil {
			return err
		}
		for _, vm := range vms {
			refs = append(refs, vm.Reference())
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find virtual machines: %w", err)
	}
	refs = dedupeRefs(refs)

	var mes []mo.VirtualMachine
	err = property.DefaultCollector(c).Retrieve(ctx, refs,
		[]string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage"},
		&mes)
	if err != nil {
		return nil, fmt.Errorf("retrieve virtual machine properties: %w", err)
	}

	out := make([]VMInfo, 0, len(mes))
	for i := range mes {
		vm := &mes[i]
		info := VMInfo{Name: vm.Name}
		if vm.Config != nil {
			info.CPUs = vm.Config.Hardware.NumCPU
			info.RAMMB = int64(vm.Config.Hardware.MemoryMB)
		}
		if vm.Summary.Storage != nil {
			info.CommittedBytes = vm.Summary.Storage.Committed
		}
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

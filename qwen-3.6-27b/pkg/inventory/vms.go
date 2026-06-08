package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VMInfo struct {
	Name    string
	VCPU    int32
	RAMMB   int32
	Storage int64
}

func ListVMs(ctx context.Context, client *govmomi.Client) ([]VMInfo, error) {
	fm := property.DefaultCollector(client.Client)
	finder := find.NewFinder(client.Client, false)

	dc, err := finder.DefaultDatacenter(ctx)
	if err == nil {
		finder.SetDatacenter(dc)
	}

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("find virtual machines: %w", err)
	}

	if len(vms) == 0 {
		return []VMInfo{}, nil
	}

	refs := make([]types.ManagedObjectReference, len(vms))
	for i, vm := range vms {
		refs[i] = vm.Reference()
	}

	var props []mo.VirtualMachine
	err = fm.Retrieve(ctx, refs, []string{"name", "config.hardware.numCPU", "config.hardware.memoryMB", "summary.storage.committed"}, &props)
	if err != nil {
		return nil, fmt.Errorf("retrieve VM properties: %w", err)
	}

	var results []VMInfo
	for _, p := range props {
		vcpu := int32(0)
		ramMB := int32(0)
		storage := int64(0)

		if p.Config != nil {
			vcpu = p.Config.Hardware.NumCPU
			ramMB = p.Config.Hardware.MemoryMB
		}
		if p.Summary.Storage != nil {
			storage = p.Summary.Storage.Committed
		}

		results = append(results, VMInfo{
			Name:    p.Name,
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

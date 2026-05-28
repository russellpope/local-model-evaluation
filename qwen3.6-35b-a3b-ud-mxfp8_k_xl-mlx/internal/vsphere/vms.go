package vsphere

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/formatter"
)

// VMInfo holds extracted information about a virtual machine.
type VMInfo struct {
	Name    string
	VCPU    int32
	RAMGB   float64
	Storage string // human-readable consumed storage
}

// ListVMs retrieves all virtual machines and returns their inventory info.
func ListVMs(ctx context.Context, client *govmomi.Client) ([]VMInfo, error) {
	finder := find.NewFinder(client.Client, false)

	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datacenters: %w", err)
	}

	if len(datacenters) == 0 {
		return nil, fmt.Errorf("no datacenters found")
	}

	finder.SetDatacenter(datacenters[0])

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list VMs: %w", err)
	}

	if len(vms) == 0 {
		return nil, nil
	}

	var vmRefs []types.ManagedObjectReference
	for _, vm := range vms {
		vmRefs = append(vmRefs, vm.Reference())
	}

	pc := client.PropertyCollector()

	var moVMs []mo.VirtualMachine
	if err := pc.Retrieve(ctx, vmRefs, []string{"Name", "Config.Hardware.NumCPU", "Config.Hardware.MemoryMB", "Summary.Storage.Committed"}, &moVMs); err != nil {
		return nil, fmt.Errorf("retrieve VM properties: %w", err)
	}

	var result []VMInfo
	for _, vm := range moVMs {
		committed := int64(0)

		if vm.Summary.Storage != nil && vm.Summary.Storage.Committed != 0 {
			committed = vm.Summary.Storage.Committed
		}

		numCPU := int32(0)
		if vm.Config != nil {
			numCPU = vm.Config.Hardware.NumCPU
		}

		memMB := int32(0)
		if vm.Config != nil {
			memMB = vm.Config.Hardware.MemoryMB
		}

		memGB := float64(memMB) / 1024.0

		result = append(result, VMInfo{
			Name:    vm.Name,
			VCPU:    numCPU,
			RAMGB:   memGB,
			Storage: formatter.FormatBytes(committed),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

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

	var allVMs []VMInfo

	for _, dc := range datacenters {
		finder.SetDatacenter(dc)

		vms, err := finder.VirtualMachineList(ctx, "*")
		if err != nil {
			continue
		}

		if len(vms) == 0 {
			continue
		}

		var vmRefs []types.ManagedObjectReference
		for _, vm := range vms {
			vmRefs = append(vmRefs, vm.Reference())
		}

		pc := client.PropertyCollector()

		var moVMs []mo.VirtualMachine
		if err := pc.Retrieve(ctx, vmRefs, []string{"summary", "summary.storage.committed"}, &moVMs); err != nil {
			continue
		}

		for i, vm := range vms {
			committed := int64(0)
			if moVMs[i].Summary.Storage != nil {
				committed = moVMs[i].Summary.Storage.Committed
			}

			numCPU := int32(0)
			memMB := int32(0)
			if moVMs[i].Summary.Config.NumCpu > 0 {
				numCPU = moVMs[i].Summary.Config.NumCpu
				memMB = int32(moVMs[i].Summary.Config.MemorySizeMB)
			}

			memGB := float64(memMB) / 1024.0

			allVMs = append(allVMs, VMInfo{
				Name:    vm.Name(),
				VCPU:    numCPU,
				RAMGB:   memGB,
				Storage: formatter.FormatBytes(committed),
			})
		}
	}

	sort.Slice(allVMs, func(i, j int) bool {
		return allVMs[i].Name < allVMs[j].Name
	})

	return allVMs, nil
}

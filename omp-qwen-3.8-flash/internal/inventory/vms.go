package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// retrieveAll fetches all managed objects of one kind across the entire
// inventory (no datacenter scoping) using a recursive container view.
func retrieveAll(ctx context.Context, c *vim25.Client, kind string, ps []string, dst any) error {
	mgr := view.NewManager(c)
	cv, err := mgr.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{kind}, true)
	if err != nil {
		return fmt.Errorf("create container view for %s: %w", kind, err)
	}
	defer func() { _ = cv.Destroy(ctx) }()
	if err := cv.Retrieve(ctx, []string{kind}, ps, dst); err != nil {
		return fmt.Errorf("retrieve %s properties: %w", kind, err)
	}
	return nil
}

// VMs returns all virtual machines in the inventory, sorted by name.
// Committed is actual consumed storage in bytes (Summary.Storage.Committed),
// not provisioned capacity.
func VMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	var vms []mo.VirtualMachine
	if err := retrieveAll(ctx, c, "VirtualMachine",
		[]string{"name", "summary.config.template", "config.hardware", "summary.storage"}, &vms); err != nil {
		return nil, fmt.Errorf("list virtual machines: %w", err)
	}

	out := make([]VMInfo, 0, len(vms))
	for _, vm := range vms {
		// Templates are not deployable inventory VMs; skip them.
		if vm.Summary.Config.Template {
			continue
		}
		info := VMInfo{Name: vm.Name}
		if vm.Config != nil {
			info.NumCPU = vm.Config.Hardware.NumCPU
			info.MemoryMB = int64(vm.Config.Hardware.MemoryMB)
		}
		if vm.Summary.Storage != nil {
			info.Committed = vm.Summary.Storage.Committed
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// retrieveOneAll fetches selected properties of a single managed object.
func retrieveOneAll(ctx context.Context, c *vim25.Client, ref types.ManagedObjectReference, ps []string, dst any) error {
	return property.DefaultCollector(c).RetrieveOne(ctx, ref, ps, dst)
}

package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// VMInfo is one row of the `vms` report. StorageBytes is consumed (committed)
// storage as reported by the API, not provisioned capacity.
type VMInfo struct {
	Name         string
	VCPU         int32
	RAMGB        float64
	StorageBytes int64
}

// ListVMs returns every virtual machine in the inventory, sorted by name.
func ListVMs(ctx context.Context, c *vim25.Client) ([]VMInfo, error) {
	v, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder,
		[]string{"VirtualMachine"}, true)
	if err != nil {
		return nil, fmt.Errorf("create VM container view: %w", err)
	}
	defer func() { _ = v.Destroy(context.Background()) }()

	var objs []mo.VirtualMachine
	if err := v.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary"}, &objs); err != nil {
		return nil, fmt.Errorf("retrieve VMs: %w", err)
	}

	out := make([]VMInfo, 0, len(objs))
	for _, vm := range objs {
		s := vm.Summary
		info := VMInfo{Name: "unknown"}
		if s.Config.Name != "" {
			info.Name = s.Config.Name
		}
		info.VCPU = s.Config.NumCpu
		info.RAMGB = float64(s.Config.MemorySizeMB) / 1024.0
		if s.Storage != nil {
			info.StorageBytes = s.Storage.Committed
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

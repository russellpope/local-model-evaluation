package inventory

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// ClassifyTransportFromDevice classifies storage transport from a device descriptor string.
func ClassifyTransportFromDevice(deviceDesc string) string {
	switch {
	case containsAny(deviceDesc, []string{"fc", "FC", "Fibre", "fibre", "fabric"}):
		return "FC"
	case containsAny(deviceDesc, []string{"nvme", "NVMe", "NVME", "nvm"}):
		return "NVMe"
	case containsAny(deviceDesc, []string{"iscsi", "iSCSI", "isc", "ISCSI"}):
		return "iSCSI"
	case containsAny(deviceDesc, []string{"nfs", "NFS"}):
		return "NFS"
	default:
		return "unknown"
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func ListDatastores(ctx context.Context, dc *object.Datacenter) ([]DatastoreInfo, error) {
	vmgr := view.NewManager(dc.Client())
	v, err := vmgr.CreateContainerView(ctx, dc.Reference(), []string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("create datastore view: %w", err)
	}
	defer v.Destroy(ctx)

	var dsList []mo.Datastore
	if err := v.Retrieve(ctx, []string{"Datastore"}, []string{"name", "summary.capacity", "summary.freeSpace", "summary.type"}, &dsList); err != nil {
		return nil, fmt.Errorf("retrieve datastores: %w", err)
	}

	var result []DatastoreInfo
	for _, ds := range dsList {
		dsType := classifyDatastoreType(ds.Summary)

		var usedGiB, availGiB, capacityGiB float64
		capacityGiB = float64(ds.Summary.Capacity) / (1024 * 1024 * 1024)
		freeGiB := float64(ds.Summary.FreeSpace) / (1024 * 1024 * 1024)
		usedGiB = capacityGiB - freeGiB
		availGiB = freeGiB

		result = append(result, DatastoreInfo{
			Name:        ds.Name,
			Type:        dsType,
			UsedGiB:     usedGiB,
			AvailGiB:    availGiB,
			CapacityGiB: capacityGiB,
		})
	}

	sortDatastoreByName(result)
	return result, nil
}

func classifyDatastoreType(summary types.DatastoreSummary) string {
	if summary.Type == "NFS" {
		return "NFS"
	}
	return "unknown"
}

func sortDatastoreByName(ds []DatastoreInfo) {
	for i := 1; i < len(ds); i++ {
		for j := i; j > 0 && ds[j].Name < ds[j-1].Name; j-- {
			ds[j], ds[j-1] = ds[j-1], ds[j]
		}
	}
}

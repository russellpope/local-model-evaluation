package vsphere

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/formatter"
	"vsphere-inventory/internal/transport"
)

// DatastoreInfo holds extracted information about a datastore.
type DatastoreInfo struct {
	Name      string
	Type      string // FC, iSCSI, NVMe, NFS, or unknown
	Used      string // human-readable
	Available string // human-readable
}

// ListDatastores retrieves all datastores and returns their inventory info.
func ListDatastores(ctx context.Context, client *govmomi.Client) ([]DatastoreInfo, error) {
	finder := find.NewFinder(client.Client, false)

	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("list datacenters: %w", err)
	}

	if len(datacenters) == 0 {
		return nil, fmt.Errorf("no datacenters found")
	}

	var allDS []*object.Datastore

	for _, dc := range datacenters {
		finder.SetDatacenter(dc)

		ds, err := finder.DatastoreList(ctx, "*")
		if err != nil {
			continue
		}

		allDS = append(allDS, ds...)
	}

	if len(allDS) == 0 {
		return nil, nil
	}

	var dsRefs []types.ManagedObjectReference
	for _, ds := range allDS {
		dsRefs = append(dsRefs, ds.Reference())
	}

	pc := client.PropertyCollector()

	var datastores []mo.Datastore
	if err := pc.Retrieve(ctx, dsRefs, []string{"summary", "Info", "self"}, &datastores); err != nil {
		return nil, fmt.Errorf("retrieve datastore properties: %w", err)
	}

	moMap := make(map[string]*mo.Datastore, len(datastores))
	for i := range datastores {
		moMap[datastores[i].Self.Value] = &datastores[i]
	}

	var result []DatastoreInfo
	for _, ds := range allDS {
		moDS, ok := moMap[ds.Reference().Value]
		if !ok {
			continue
		}

		capacity := moDS.Summary.Capacity
		freeSpace := moDS.Summary.FreeSpace

		used := capacity - freeSpace
		if used < 0 {
			used = 0
		}

		dsType := classifyTransport(*moDS)

		result = append(result, DatastoreInfo{
			Name:      ds.Name(),
			Type:      dsType,
			Used:      formatter.FormatBytes(used),
			Available: formatter.FormatBytes(freeSpace),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// classifyTransport determines the storage transport type from a datastore's backing info.
func classifyTransport(ds mo.Datastore) string {
	if ds.Info == nil {
		return "unknown"
	}

	switch info := ds.Info.(type) {
	case *types.NasDatastoreInfo:
		return "NFS"
	case *types.VmfsDatastoreInfo:
		if info.Vmfs != nil && len(info.Vmfs.Extent) > 0 {
			for _, extent := range info.Vmfs.Extent {
				if extent.DiskName != "" {
					return transport.Classify(extent.DiskName)
				}
			}
		}
		return "unknown"
	default:
		return "unknown"
	}
}

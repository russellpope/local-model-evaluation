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

	finder.SetDatacenter(datacenters[0])

	allDS, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("find datastores: %w", err)
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
	if err := pc.Retrieve(ctx, dsRefs, []string{"Name", "Summary"}, &datastores); err != nil {
		return nil, fmt.Errorf("retrieve datastore properties: %w", err)
	}

	var dsBacking []mo.Datastore
	if err := pc.Retrieve(ctx, dsRefs, []string{"Info"}, &dsBacking); err != nil {
		return nil, fmt.Errorf("retrieve datastore backing info: %w", err)
	}

	var result []DatastoreInfo
	for i, ds := range datastores {
		capacity := int64(0)
		freeSpace := int64(0)

		capacity = ds.Summary.Capacity
		freeSpace = ds.Summary.FreeSpace

		used := capacity - freeSpace
		if used < 0 {
			used = 0
		}

		dsType := classifyTransport(dsBacking, i)

		result = append(result, DatastoreInfo{
			Name:      ds.Name,
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
func classifyTransport(dsBacking []mo.Datastore, index int) string {
	if len(dsBacking) <= index || dsBacking[index].Info == nil {
		return "unknown"
	}

	switch info := dsBacking[index].Info.(type) {
	case *types.NasDatastoreInfo:
		return "NFS"
	case *types.VmfsDatastoreInfo:
		if info.Vmfs != nil && len(info.Vmfs.Extent) > 0 {
			for _, extent := range info.Vmfs.Extent {
				if diskDevice := extractDiskDevice(extent.DiskName); diskDevice != "" {
					return transport.Classify(diskDevice)
				}
			}
		}
		return "unknown"
	default:
		return "unknown"
	}
}

// extractDiskDevice extracts the disk device identifier from a VMFS extent's disk name.
func extractDiskDevice(diskName string) string {
	if diskName == "" {
		return ""
	}
	return diskName
}

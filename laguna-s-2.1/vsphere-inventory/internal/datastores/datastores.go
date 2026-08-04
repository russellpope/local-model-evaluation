package datastores

import (
	"context"
	"fmt"
	"os"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/transport"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func GetDatastores(ctx context.Context, client *vim25.Client) ([]format.DatastoreInfo, error) {
	finder := find.NewFinder(client)

	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datacenters: %w", err)
	}

	cache := transport.NewHostCache(client)
	if err := cache.PrefetchAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "warning: pre-fetching host cache: %v\n", err)
	}

	var result []format.DatastoreInfo
	for _, dc := range dcs {
		finder.SetDatacenter(dc)

		datastores, err := finder.DatastoreList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing datastores in datacenter %s: %w", dc.Name(), err)
		}

		var dsRefs []types.ManagedObjectReference
		for _, ds := range datastores {
			dsRefs = append(dsRefs, ds.Reference())
		}

		if len(dsRefs) == 0 {
			continue
		}

		var dsMos []mo.Datastore
		pc := property.DefaultCollector(client)
		if err := pc.Retrieve(ctx, dsRefs, []string{
			"name",
			"summary.capacity",
			"summary.freeSpace",
			"info",
			"host",
		}, &dsMos); err != nil {
			return nil, fmt.Errorf("retrieving datastore properties: %w", err)
		}

		for i := range dsMos {
			dsMo := dsMos[i]
			capacity := dsMo.Summary.Capacity
			freeSpace := dsMo.Summary.FreeSpace

			used := capacity - freeSpace

			classified, err := transport.ClassifyDatastoreWithCache(ctx, client, dsMo, cache)
			if err != nil {
				return nil, fmt.Errorf("classifying datastore %s: %w", dsMo.Name, err)
			}

			if classified.Reason != "" {
				fmt.Fprintf(os.Stderr, "warning: datastore %s: %s\n", dsMo.Name, classified.Reason)
			}

			result = append(result, format.DatastoreInfo{
				Name:           dsMo.Name,
				Type:           classified.Type,
				UsedBytes:      used,
				AvailableBytes: freeSpace,
				CapacityBytes:  capacity,
			})
		}
	}

	return result, nil
}

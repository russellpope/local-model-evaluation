package datastores

import (
	"context"
	"fmt"
	"os"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/format"
	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/transport"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
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

		for _, ds := range datastores {
			var dsMo mo.Datastore
			err := ds.Properties(ctx, ds.Reference(), []string{
				"name",
				"summary.capacity",
				"summary.freeSpace",
				"info",
				"host",
			}, &dsMo)
			if err != nil {
				return nil, fmt.Errorf("retrieving properties for datastore %s: %w", ds.Name(), err)
			}

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

package datastores

import (
	"context"
	"fmt"

	"github.com/local-model-evaluation/laguna-s-2.1/vsphere-inventory/internal/transport"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

type DatastoreInfo struct {
	Name           string
	Type           string
	UsedBytes      int64
	AvailableBytes int64
	CapacityBytes  int64
}

func GetDatastores(ctx context.Context, client *vim25.Client) ([]DatastoreInfo, error) {
	finder := find.NewFinder(client)

	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("listing datacenters: %w", err)
	}

	var result []DatastoreInfo
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
				"summary.type",
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

			transportType, err := transport.ClassifyDatastore(ctx, client, dsMo)
			if err != nil {
				return nil, fmt.Errorf("classifying datastore %s: %w", dsMo.Name, err)
			}

			result = append(result, DatastoreInfo{
				Name:           dsMo.Name,
				Type:           transportType,
				UsedBytes:      used,
				AvailableBytes: freeSpace,
				CapacityBytes:  capacity,
			})
		}
	}

	return result, nil
}

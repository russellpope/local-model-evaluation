package datastores

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/qwen3-coder-next/internal/model"
)

func ListDatastores(ctx context.Context, client *vim25.Client) ([]model.DatastoreInfo, error) {
	finder := find.NewFinder(client, false)
	
	dc, err := finder.DefaultDatacenter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get default datacenter: %w", err)
	}
	
	finder.SetDatacenter(dc)
	
	dsList, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to list datastores: %w", err)
	}
	
	var dss []model.DatastoreInfo
	for _, ds := range dsList {
		var props struct {
			Name       string
			Summary    types.DatastoreSummary
			Info       types.DatastoreInfo
		}
		
		pc := property.DefaultCollector(client)
		if err := pc.RetrieveOne(ctx, ds.Reference(), []string{"name", "summary.capacity", "summary.freeSpace", "info"}, &props); err != nil {
			return nil, fmt.Errorf("failed to get datastore properties: %w", err)
		}
		
		info := model.DatastoreInfo{
			Name:        props.Name,
			Capacity:    int64(props.Summary.Capacity),
			Used:        int64(props.Summary.Capacity - props.Summary.FreeSpace),
			Available:   int64(props.Summary.FreeSpace),
			Type:        model.ClassifyTransport(nil),
		}
		
		dss = append(dss, info)
	}
	
	return dss, nil
}

func init() {
	_ = find.NewFinder
	_ = property.DefaultCollector
}

package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"govmomi-cli/pkg/inventory/utils"
)

type DatastoreInfo struct {
	Name      string
	Type      string
	Used      string
	Available string
}

func GetDatastores(ctx context.Context, client *vim25.Client) ([]DatastoreInfo, error) {
	view, err := getDatastoreView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer view.Destroy(ctx)

	var dstores []mo.Datastore
	err = view.Retrieve(ctx, []string{"Datastore"}, []string{"name", "summary.capacity", "summary.freeSpace", "info"}, &dstores)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve datastore properties: %w", err)
	}

	var result []DatastoreInfo
	for _, ds := range dstores {
		capacity := int64(ds.Summary.Capacity)
		free := int64(ds.Summary.FreeSpace)
		used := capacity - free

		result = append(result, DatastoreInfo{
			Name:      ds.Name,
			Type:      classifyTransport(ds),
			Used:      utils.FormatBytes(used),
			Available: utils.FormatBytes(free),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func classifyTransport(ds mo.Datastore) string {
	// In a real app, we'd inspect the backing.
	return "unknown"
}

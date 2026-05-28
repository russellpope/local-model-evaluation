package vim

import (
	"context"
	"fmt"

	"github.com/vmware/govmomi/vim25/mo"
)

type DatastoreInfo struct {
	Name      string
	Transport string
	Capacity  int64
	Used      int64
	Available int64
}

func (c *Client) GetDatastores(ctx context.Context) ([]DatastoreInfo, error) {
	v, err := c.View.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("creating datastore view: %w", err)
	}
	defer v.Destroy(ctx)

	var dss []mo.Datastore

	err = v.Retrieve(ctx, []string{"Datastore"}, nil, &dss)
	if err != nil {
		return nil, fmt.Errorf("retrieving datastores: %w", err)
	}

	info := make([]DatastoreInfo, 0, len(dss))
	for _, ds := range dss {
		dsInfo := DatastoreInfo{
			Name:      ds.Summary.Name,
			Capacity:  ds.Summary.Capacity,
			Used:      ds.Summary.Capacity - ds.Summary.FreeSpace,
			Available: ds.Summary.FreeSpace,
		}

		dsInfo.Transport = classifyTransport(ds.Summary.Type)

		info = append(info, dsInfo)
	}

	return info, nil
}

func classifyTransport(dsType string) string {
	switch dsType {
	case "nfs", "nfs41":
		return "NFS"
	case "vsan":
		return "NVMe"
	default:
		return "unknown"
	}
}

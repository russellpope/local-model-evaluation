package inventory

import (
	"context"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/view"
	"github.com/local-model-evaluation/muse-glimmer-30b-bf16/vsphere-cli/internal/output"
)

func GetDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	m := view.NewManager(c)
	v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Datastore"}, true)
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)
	var ds []mo.Datastore
	err = v.Retrieve(ctx, []string{"Datastore"}, []string{"name", "summary.capacity", "summary.freeSpace", "summary.type"}, &ds)
	if err != nil {
		return nil, err
	}
	results := make([]DatastoreInfo, 0, len(ds))
	for _, d := range ds {
		name := d.Name
		typ := ClassifyTransport(name, d.Summary.Type)
		if d.Summary.Type == "NFS" {
			typ = "NFS"
		}
		capacity := d.Summary.Capacity
		free := d.Summary.FreeSpace
		used := capacity - free
		if used < 0 {
			used = 0
		}
		results = append(results, DatastoreInfo{
			Name:      name,
			Type:      typ,
			Used:      output.BytesToHuman(used),
			Available: output.BytesToHuman(free),
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Name < results[j].Name })
	return results, nil
}

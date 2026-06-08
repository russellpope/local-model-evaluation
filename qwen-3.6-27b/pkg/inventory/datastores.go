package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type DatastoreInfo struct {
	Name      string
	Type      string
	Used      int64
	Available int64
	Capacity  int64
}

func ListDatastores(ctx context.Context, client *govmomi.Client) ([]DatastoreInfo, error) {
	fm := property.DefaultCollector(client.Client)
	finder := find.NewFinder(client.Client, false)

	dc, err := finder.DefaultDatacenter(ctx)
	if err == nil {
		finder.SetDatacenter(dc)
	}

	dss, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("find datastores: %w", err)
	}

	if len(dss) == 0 {
		return []DatastoreInfo{}, nil
	}

	refs := make([]types.ManagedObjectReference, len(dss))
	for i, ds := range dss {
		refs[i] = ds.Reference()
	}

	var props []mo.Datastore
	err = fm.Retrieve(ctx, refs, []string{"name", "info", "summary"}, &props)
	if err != nil {
		return nil, fmt.Errorf("retrieve datastore properties: %w", err)
	}

	var results []DatastoreInfo
	for _, p := range props {
		dsType := classifyDatastoreTransport(&p)

		used := int64(0)
		available := p.Summary.FreeSpace
		capacity := p.Summary.Capacity
		used = capacity - available

		results = append(results, DatastoreInfo{
			Name:      p.Name,
			Type:      dsType,
			Used:      used,
			Available: available,
			Capacity:  capacity,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results, nil
}

func classifyDatastoreTransport(ds *mo.Datastore) string {
	if ds.Info == nil {
		return "unknown"
	}

	switch info := ds.Info.(type) {
	case *types.NasDatastoreInfo:
		return "NFS"
	case *types.VmfsDatastoreInfo:
		return classifyVMFSBacking(info)
	case *types.LocalDatastoreInfo:
		return "local"
	case *types.VsanDatastoreInfo:
		return "VSAN"
	case *types.PMemDatastoreInfo:
		return "NVMe"
	case *types.VvolDatastoreInfo:
		return "VVOL"
	default:
		return "unknown"
	}
}

func classifyVMFSBacking(info *types.VmfsDatastoreInfo) string {
	if info == nil {
		return "unknown"
	}

	if info.Url != "" {
		return classifyByUUID(info.Url)
	}

	return "unknown"
}

func classifyByUUID(uuid string) string {
	if strings.HasPrefix(uuid, "60") {
		return "FC"
	}
	return "unknown"
}

// ClassifyTransport is a pure function for testing the transport classification logic.
func ClassifyTransport(hbaType string) string {
	return classifyHBA(hbaType)
}

func classifyHBA(transport string) string {
	switch strings.ToLower(transport) {
	case "fc", "fcqe":
		return "FC"
	case "iscsi", "softwareiscsi", "hardwareiscsi":
		return "iSCSI"
	case "nvme":
		return "NVMe"
	case "sata":
		return "SATA"
	case "usb":
		return "USB"
	default:
		return "unknown"
	}
}

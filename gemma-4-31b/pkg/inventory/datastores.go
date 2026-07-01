package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"govmomi-cli/pkg/inventory/utils"
)

type DatastoreInfo struct {
	Name      string
	Type      string
	Used      string
	Available string
}

type TransportDescriptor struct {
	SummaryType string
	Info        interface{}
	AdapterInfo string
}

func GetDatastores(ctx context.Context, client *vim25.Client) ([]DatastoreInfo, error) {
	view, err := getDatastoreView(ctx, client)
	if err != nil {
		return nil, err
	}
	defer view.Destroy(ctx)

	var dstores []mo.Datastore
	err = view.Retrieve(ctx, []string{"Datastore"}, []string{"name", "summary.capacity", "summary.freeSpace", "summary.type", "info"}, &dstores)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve datastore properties: %w", err)
	}

	var result []DatastoreInfo
	for _, ds := range dstores {
		capacity := int64(ds.Summary.Capacity)
		free := int64(ds.Summary.FreeSpace)
		used := capacity - free

		desc := TransportDescriptor{
			SummaryType: ds.Summary.Type,
			Info:        ds.Info,
		}

		result = append(result, DatastoreInfo{
			Name:      ds.Name,
			Type:      classifyTransport(desc),
			Used:      utils.FormatBytes(used),
			Available: utils.FormatBytes(free),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func classifyTransport(desc TransportDescriptor) string {
	if desc.Info != nil {
		switch desc.Info.(type) {
		case *types.NasDatastoreInfo:
			return "NFS"
		case *types.VmfsDatastoreInfo:
			if desc.AdapterInfo != "" {
				return classifyAdapter(desc.AdapterInfo)
			}
			return "unknown"
		}
	}
	if desc.SummaryType == "NFS" || desc.SummaryType == "NFS41" {
		return "NFS"
	}
	return "unknown"
}

func classifyAdapter(adapterInfo string) string {
	lower := strings.ToLower(adapterInfo)
	if strings.Contains(lower, "fc") || strings.Contains(lower, "fibre channel") {
		return "FC"
	}
	if strings.Contains(lower, "iscsi") {
		return "iSCSI"
	}
	if strings.Contains(lower, "nvme") {
		return "NVMe"
	}
	return "unknown"
}

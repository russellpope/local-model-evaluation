package storage

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
)

type DatastoreInfo struct {
	Name      string
	Type      string // FC, iSCSI, NVMe, NFS, or unknown
	UsedGB    float64
	AvailableGB float64
}

func ClassifyTransport(url string) string {
	urlLower := strings.ToLower(url)
	if strings.Contains(urlLower, "nfs") {
		return "NFS"
	}
	if strings.Contains(urlLower, "vmfs") {
		return "unknown"
	}
	if strings.Contains(urlLower, "iscsi") || strings.Contains(urlLower, "iSCSI") {
		return "iSCSI"
	}
	if strings.Contains(urlLower, "nvme") || strings.Contains(urlLower, "NVMe") {
		return "NVMe"
	}
	if strings.Contains(urlLower, "fc") || strings.Contains(urlLower, "FC") {
		return "FC"
	}
	return "unknown"
}

func GetDatastores(ctx context.Context, client *govmomi.Client) ([]DatastoreInfo, error) {
	finder := find.NewFinder(client.Client, false)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("finding datacenters: %w", err)
	}

	var allDatastores []DatastoreInfo

	for _, dc := range datacenters {
		finder.SetDatacenter(dc)
		datastores, err := finder.DatastoreList(ctx, "*")
		if err != nil {
			return nil, fmt.Errorf("listing datastores in datacenter %s: %w", dc.Name(), err)
		}

		for _, ds := range datastores {
			info, err := getDatastoreInfo(ctx, ds)
			if err != nil {
				continue
			}
			allDatastores = append(allDatastores, info)
		}
	}

	sort.Slice(allDatastores, func(i, j int) bool {
		return allDatastores[i].Name < allDatastores[j].Name
	})

	return allDatastores, nil
}

func getDatastoreInfo(ctx context.Context, ds *object.Datastore) (DatastoreInfo, error) {
	// Get the datastore properties using the Properties method
	var dsMo mo.Datastore
	err := ds.Properties(ctx, ds.Reference(), []string{"name", "summary"}, &dsMo)
	if err != nil {
		return DatastoreInfo{}, fmt.Errorf("getting datastore properties: %w", err)
	}

	name := dsMo.Name
	summary := dsMo.Summary

	// Determine transport type from URL
	transport := ClassifyTransport(ds.InventoryPath)

	// Calculate used space: capacity - free
	usedBytes := int64(0)
	if summary.Capacity > 0 && summary.FreeSpace > 0 {
		usedBytes = summary.Capacity - summary.FreeSpace
	}
	freeBytes := summary.FreeSpace

	return DatastoreInfo{
		Name:        name,
		Type:        transport,
		UsedGB:      float64(usedBytes) / (1024 * 1024 * 1024),
		AvailableGB: float64(freeBytes) / (1024 * 1024 * 1024),
	}, nil
}

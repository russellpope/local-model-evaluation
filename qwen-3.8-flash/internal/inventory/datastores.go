package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
)

// DatastoreInfo is one row of the datastores table.
type DatastoreInfo struct {
	Name      string
	Transport string // FC, iSCSI, NVMe, NFS or unknown
	Capacity  int64
	FreeSpace int64
}

// ListDatastores returns every datastore with its derived transport, sorted
// by name.
func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	refs, err := listKind(ctx, c, "Datastore")
	if err != nil {
		return nil, err
	}
	var stores []mo.Datastore
	if err := collect(ctx, c, refs, []string{"name", "summary", "info", "host"}, &stores); err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}

	// Host storage topology is needed to derive non-NFS transports.
	var hosts []mo.HostSystem
	needHosts := false
	for i := range stores {
		if !isNFSType(stores[i].Summary.Type) {
			needHosts = true
		}
	}
	if needHosts {
		hostRefs, err := listKind(ctx, c, "HostSystem")
		if err != nil {
			return nil, err
		}
		if err := collect(ctx, c, hostRefs, []string{"name", "config"}, &hosts); err != nil {
			return nil, fmt.Errorf("list host storage devices: %w", err)
		}
	}

	out := make([]DatastoreInfo, 0, len(stores))
	for i := range stores {
		ds := &stores[i]
		out = append(out, DatastoreInfo{
			Name:      ds.Summary.Name,
			Transport: resolveDatastoreTransport(ds, hosts),
			Capacity:  ds.Summary.Capacity,
			FreeSpace: ds.Summary.FreeSpace,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

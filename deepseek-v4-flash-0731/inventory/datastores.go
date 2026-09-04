package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"vint/transport"
)

// DatastoreInfo is the typed result for the datastores subcommand. Capacity,
// Used and Free are byte counts; Type is a transport protocol (FC/iSCSI/NVMe/
// NFS) or "unknown".
type DatastoreInfo struct {
	Name     string
	Type     string
	Capacity int64
	Used     int64
	Free     int64
}

// UsedCapacity computes consumed capacity as capacity minus free space,
// clamped at zero so rounding/API skew can never produce a negative value.
func UsedCapacity(capacity, free int64) int64 {
	if u := capacity - free; u > 0 {
		return u
	}
	return 0
}

// ListDatastores returns all datastores sorted by name.
func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	var refs []types.ManagedObjectReference
	err := withDatacenters(ctx, c, func(f *find.Finder) error {
		dss, err := f.DatastoreList(ctx, "*")
		if err != nil {
			return err
		}
		for _, ds := range dss {
			refs = append(refs, ds.Reference())
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find datastores: %w", err)
	}
	refs = dedupeRefs(refs)

	var mes []mo.Datastore
	err = property.DefaultCollector(c).Retrieve(ctx, refs,
		[]string{"name", "summary.capacity", "summary.freeSpace", "info", "host"},
		&mes)
	if err != nil {
		return nil, fmt.Errorf("retrieve datastore properties: %w", err)
	}

	out := make([]DatastoreInfo, 0, len(mes))
	for i := range mes {
		ds := &mes[i]
		info := DatastoreInfo{
			Name:     ds.Name,
			Type:     DatastoreTransport(ctx, c, ds),
			Capacity: ds.Summary.Capacity,
			Free:     ds.Summary.FreeSpace,
		}
		info.Used = UsedCapacity(info.Capacity, info.Free)
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// DatastoreTransport derives the storage transport backing a datastore.
//
// The datastore filesystem type (VMFS/NFS) is deliberately NOT used: a single
// VMFS datastore may sit on FC, iSCSI or NVMe behind the scenes. Derivation
// order:
//
//  1. An NFS datastore is reported via its DatastoreInfo (NasDatastoreInfo).
//  2. A VMFS datastore: classify the backing LUN identifier(s) from the
//     volume's extents (e.g. "naa.…" → FC, "t10.NVMe____…" → NVMe).
//  3. Fall back to scanning the mounted hosts' HBA descriptors.
//  4. Whatever we cannot prove is "unknown".
func DatastoreTransport(ctx context.Context, c *vim25.Client, ds *mo.Datastore) string {
	if t := transportFromInfo(ds.Info); t != transport.Unknown {
		return t
	}
	return transportFromHosts(ctx, c, ds.Host)
}

func transportFromInfo(info types.BaseDatastoreInfo) string {
	switch i := info.(type) {
	case *types.NasDatastoreInfo:
		return transport.NFS
	case *types.VmfsDatastoreInfo:
		for _, ext := range i.Vmfs.Extent {
			if t := transport.Classify(ext.DiskName); t != transport.Unknown {
				return t
			}
		}
	}
	return transport.Unknown
}

func transportFromHosts(ctx context.Context, c *vim25.Client, hosts []types.DatastoreHostMount) string {
	pc := property.DefaultCollector(c)
	for _, mount := range hosts {
		var h mo.HostSystem
		err := pc.RetrieveOne(ctx, mount.Key, []string{"config.storageDevice.hostBusAdapter"}, &h)
		if err != nil || h.Config == nil || h.Config.StorageDevice == nil {
			continue
		}
		for _, adapter := range h.Config.StorageDevice.HostBusAdapter {
			if t := transport.Classify(adapterDescriptor(adapter)); t != transport.Unknown {
				return t
			}
		}
	}
	return transport.Unknown
}

func adapterDescriptor(a types.BaseHostHostBusAdapter) string {
	h := a.GetHostHostBusAdapter()
	if h.Driver != "" {
		return h.Driver + " " + h.Model
	}
	return h.Model
}

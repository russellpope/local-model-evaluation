package inventory

import (
	"context"
	"fmt"
	"sort"

	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// DatastoreInfo is one row of the `datastores` report. Type is the backing
// transport (FC, iSCSI, NVMe or NFS), not the filesystem type.
type DatastoreInfo struct {
	Name           string
	Type           string
	CapacityBytes  int64
	UsedBytes      int64
	AvailableBytes int64
}

// ListDatastores returns every datastore in the inventory with used/available
// capacity and its real storage transport, sorted by name.
func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	lunKinds, err := lunTransportMap(ctx, c)
	if err != nil {
		return nil, err
	}

	v, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder,
		[]string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("create datastore container view: %w", err)
	}
	defer func() { _ = v.Destroy(context.Background()) }()

	var objs []mo.Datastore
	if err := v.Retrieve(ctx, []string{"Datastore"}, []string{"name", "info", "summary"}, &objs); err != nil {
		return nil, fmt.Errorf("retrieve datastores: %w", err)
	}

	out := make([]DatastoreInfo, 0, len(objs))
	for _, ds := range objs {
		info := DatastoreInfo{
			Name:           ds.Name,
			CapacityBytes:  ds.Summary.Capacity,
			AvailableBytes: ds.Summary.FreeSpace,
		}
		info.UsedBytes = UsedOf(ds.Summary.Capacity, ds.Summary.FreeSpace)
		info.Type = classifyDatastoreInfo(ds.Info, lunKinds)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// classifyDatastoreInfo derives the transport for one datastore from its info
// object: NAS datastores are NFS by definition; block (VMFS) datastores are
// classified through their extent disks' HBA paths; anything else is unknown.
func classifyDatastoreInfo(info types.BaseDatastoreInfo, lunKinds map[string]string) string {
	switch i := info.(type) {
	case *types.NasDatastoreInfo:
		return "NFS" // covers NFSv3 ("NFS") and NFSv4.1 ("NFS41")
	case *types.VmfsDatastoreInfo:
		kinds := make([]string, 0, len(i.Vmfs.Extent))
		for _, ext := range i.Vmfs.Extent {
			if k, ok := lunKinds[ext.DiskName]; ok {
				kinds = append(kinds, k)
			}
		}
		return ClassifyTransport(kinds)
	default:
		return "unknown"
	}
}

// lunTransportMap scans all hosts' storage devices and returns a map from LUN
// canonical name to fabric kind (see ClassifyTransport).
func lunTransportMap(ctx context.Context, c *vim25.Client) (map[string]string, error) {
	v, err := view.NewManager(c).CreateContainerView(ctx, c.ServiceContent.RootFolder,
		[]string{"HostSystem"}, true)
	if err != nil {
		return nil, fmt.Errorf("create host container view: %w", err)
	}
	defer func() { _ = v.Destroy(context.Background()) }()

	var hosts []mo.HostSystem
	if err := v.Retrieve(ctx, []string{"HostSystem"}, []string{"config.storageDevice"}, &hosts); err != nil {
		return nil, fmt.Errorf("retrieve host storage devices: %w", err)
	}

	out := make(map[string]string)
	for _, h := range hosts {
		if h.Config == nil || h.Config.StorageDevice == nil {
			continue
		}
		dev := h.Config.StorageDevice

		canonical := make(map[string]string, len(dev.ScsiLun))
		for _, l := range dev.ScsiLun {
			canonical[l.GetScsiLun().Key] = l.GetScsiLun().CanonicalName
		}

		hbaKinds := make(map[string]string)
		for _, hba := range dev.HostBusAdapter {
			if k := hbaKind(hba); k != "" {
				hbaKinds[hba.GetHostHostBusAdapter().Key] = k
			}
		}

		if dev.ScsiTopology == nil {
			continue
		}
		for _, adapter := range dev.ScsiTopology.Adapter {
			adapterKind := hbaKinds[adapter.Adapter]
			for _, target := range adapter.Target {
				// An explicit target transport is more specific than the
				// adapter kind; fall back to the adapter kind otherwise.
				kind := adapterKind
				if tk := targetTransportKind(target.Transport); tk != "" {
					kind = tk
				}
				if kind == "" {
					continue
				}
				for _, lun := range target.Lun {
					if cn, ok := canonical[lun.ScsiLun]; ok && cn != "" {
						out[cn] = kind
					}
				}
			}
		}
	}
	return out, nil
}

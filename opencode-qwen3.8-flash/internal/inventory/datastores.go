package inventory

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/vmware/govmomi/object"

	"github.com/local-model-evaluation/vsphere-inventory/internal/format"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// DatastoreInfo holds the per-datastore fields shown by the datastores
// subcommand.
type DatastoreInfo struct {
	Name string
	// Type is the storage transport: FC, iSCSI, NVMe, NFS, or unknown.
	Type      string
	Capacity  int64
	Used      int64
	Available int64
}

// FetchDatastores returns every datastore, sorted by name, with the transport
// derived from the backing storage devices rather than the filesystem type.
func FetchDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	var dss []mo.Datastore
	if err := retrieve(ctx, c, "Datastore", []string{"name", "summary", "host"}, &dss); err != nil {
		return nil, err
	}

	// The Datastore "info" property (added in vSphere 8) is the only source of
	// VMFS extent disk names. Older servers may reject it; degrade to
	// unknown transport instead of failing the whole listing.
	var infos []mo.Datastore
	infoByName := map[string]types.BaseDatastoreInfo{}
	if err := retrieve(ctx, c, "Datastore", []string{"name", "info"}, &infos); err == nil {
		for i := range infos {
			infoByName[infos[i].Name] = infos[i].Info
		}
	}

	cache := map[string]*hostStorageIndex{}
	out := make([]DatastoreInfo, 0, len(dss))
	for i := range dss {
		ds := &dss[i]
		row := DatastoreInfo{Name: ds.Name}
		if row.Name == "" {
			row.Name = ds.Summary.Name
		}
		capacity, free := ds.Summary.Capacity, ds.Summary.FreeSpace
		fsType := ds.Summary.Type
		var extents []types.HostScsiDiskPartition

		switch info := infoByName[row.Name].(type) {
		case *types.VmfsDatastoreInfo:
			if info.Vmfs != nil {
				fsType = info.Vmfs.Type
				extents = info.Vmfs.Extent
				if info.Vmfs.Capacity > 0 {
					capacity = info.Vmfs.Capacity
				}
			}
			if info.FreeSpace > 0 {
				free = info.FreeSpace
			}
		case *types.NasDatastoreInfo:
			if info.Nas != nil {
				fsType = info.Nas.Type
			}
			if info.FreeSpace > 0 {
				free = info.FreeSpace
			}
		case *types.LocalDatastoreInfo:
			if info.FreeSpace > 0 {
				free = info.FreeSpace
			}
		}
		// Summary data is optional; never report more free than capacity.
		if capacity > 0 && free > capacity {
			free = capacity
		}

		row.Type = datastoresTransport(ctx, c, fsType, extents, hostRefs(ds), cache)
		row.Capacity = capacity
		row.Available = free
		row.Used = format.Used(capacity, free)
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func hostRefs(ds *mo.Datastore) []types.ManagedObjectReference {
	refs := make([]types.ManagedObjectReference, 0, len(ds.Host))
	for _, h := range ds.Host {
		refs = append(refs, h.Key)
	}
	return refs
}

// datastoresTransport derives the real storage transport. NFS file systems
// report NFS; VMFS extents are traced through each attached host's storage
// topology down to the owning host bus adapter. Fields the server does not
// populate degrade to unknown.
func datastoresTransport(ctx context.Context, c *vim25.Client, fsType string, extents []types.HostScsiDiskPartition, hosts []types.ManagedObjectReference, cache map[string]*hostStorageIndex) string {
	if strings.HasPrefix(strings.ToUpper(fsType), "NFS") {
		return TransportNFS
	}
	if len(extents) == 0 {
		return TransportUnknown
	}
	for _, ref := range hosts {
		idx, err := hostStorageIndexFor(ctx, c, ref, cache)
		if err != nil {
			continue
		}
		for _, ext := range extents {
			if t := idx.transportFor(ext.DiskName); t != TransportUnknown {
				return t
			}
		}
	}
	return TransportUnknown
}

// hostStorageIndex maps a host's storage device names to the transport of
// their owning adapters.
type hostStorageIndex struct {
	byLun       map[string]string   // LUN identifier -> adapter device (vmhbaN)
	byNvme      map[string]struct{} // NVMe namespace names
	adapterType map[string]string   // adapter device -> concrete type name
}

func (idx *hostStorageIndex) transportFor(diskName string) string {
	if _, ok := idx.byNvme[diskName]; ok {
		return TransportNVMe
	}
	if adapter, ok := idx.byLun[diskName]; ok {
		if typeName, ok := idx.adapterType[adapter]; ok {
			return ClassifyTransport(typeName)
		}
		return ClassifyTransport(adapter)
	}
	return ClassifyTransport(diskName)
}

var vmhbaRe = regexp.MustCompile(`vmhba\d+`)

func buildHostStorageIndex(sd *types.HostStorageDeviceInfo) *hostStorageIndex {
	idx := &hostStorageIndex{
		byLun:       map[string]string{},
		byNvme:      map[string]struct{}{},
		adapterType: map[string]string{},
	}
	if sd == nil {
		return idx
	}
	for _, a := range sd.HostBusAdapter {
		base := a.GetHostHostBusAdapter()
		idx.adapterType[base.Device] = fmt.Sprintf("%T", a)
	}
	for _, l := range sd.ScsiLun {
		lun := l.GetScsiLun()
		adapter := ""
		ids := make([]string, 0, len(lun.Descriptor)+3)
		for _, d := range lun.Descriptor {
			ids = append(ids, d.Id)
			if adapter == "" {
				adapter = vmhbaFrom(d.Id)
			}
		}
		ids = append(ids, lun.CanonicalName, lun.Uuid)
		if adapter == "" {
			adapter = vmhbaFrom(lun.CanonicalName)
		}
		if adapter == "" && lun.HostDevice.DeviceName != "" {
			ids = append(ids, lun.HostDevice.DeviceName)
			adapter = vmhbaFrom(lun.HostDevice.DeviceName)
		}
		if adapter == "" {
			continue
		}
		for _, id := range ids {
			if id != "" {
				idx.byLun[id] = adapter
			}
		}
	}
	if sd.NvmeTopology != nil {
		for _, iface := range sd.NvmeTopology.Adapter {
			for _, ctrl := range iface.ConnectedController {
				for _, ns := range ctrl.AttachedNamespace {
					if ns.Name != "" {
						idx.byNvme[ns.Name] = struct{}{}
					}
				}
			}
		}
	}
	return idx
}

func vmhbaFrom(s string) string {
	return vmhbaRe.FindString(s)
}

func hostStorageIndexFor(ctx context.Context, c *vim25.Client, ref types.ManagedObjectReference, cache map[string]*hostStorageIndex) (*hostStorageIndex, error) {
	if idx, ok := cache[ref.Value]; ok {
		return idx, nil
	}
	var hs mo.HostSystem
	if err := object.NewHostSystem(c, ref).Properties(ctx, ref, []string{"config.storageDevice"}, &hs); err != nil {
		return nil, fmt.Errorf("retrieve storage topology of host %s: %w", ref.Value, err)
	}
	if hs.Config == nil {
		return nil, fmt.Errorf("host %s exposes no storage configuration", ref.Value)
	}
	idx := buildHostStorageIndex(hs.Config.StorageDevice)
	cache[ref.Value] = idx
	return idx, nil
}

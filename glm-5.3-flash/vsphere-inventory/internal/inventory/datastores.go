package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/format"
)

// ListDatastores returns every datastore, sorted by name. USED is derived as
// capacity minus available; TYPE is the backing transport derived from the
// backing devices' HBAs (FC/iSCSI/NVMe), or NFS for network volumes,
// or "unknown" when the transport cannot be determined.
func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	v, err := view.NewManager(c).CreateContainerView(
		ctx, c.ServiceContent.RootFolder, []string{"Datastore"}, true)
	if err != nil {
		return nil, fmt.Errorf("create container view for datastores: %w", err)
	}
	defer func() { _ = v.Destroy(context.WithoutCancel(ctx)) }()

	var dss []mo.Datastore
	err = v.Retrieve(ctx, []string{"Datastore"},
		[]string{"name", "summary", "info", "host"}, &dss)
	if err != nil {
		return nil, fmt.Errorf("retrieve datastores: %w", err)
	}

	pc := property.DefaultCollector(c)
	facts := newStorageFactCache()

	out := make([]DatastoreInfo, 0, len(dss))
	for i := range dss {
		ds := &dss[i]
		info := DatastoreInfo{
			Name:           ds.Summary.Name,
			CapacityBytes:  ds.Summary.Capacity,
			AvailableBytes: ds.Summary.FreeSpace,
			UsedBytes:      format.UsedCapacity(ds.Summary.Capacity, ds.Summary.FreeSpace),
		}
		info.Type = classifyDatastore(ctx, pc, ds, facts)
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// classifyDatastore picks the transport for one datastore. NFS is reported
// directly from the datastore type; VMFS-family datastores are resolved
// through the attached hosts' storage topology. Lookup failures never fail
// the listing — they degrade to "unknown".
func classifyDatastore(ctx context.Context, pc *property.Collector, ds *mo.Datastore, facts *storageFactCache) string {
	if strings.HasPrefix(strings.ToUpper(ds.Summary.Type), "NFS") {
		return TransportNFS
	}
	if _, ok := ds.Info.(*types.NasDatastoreInfo); ok {
		return TransportNFS
	}

	volumeUUID := ""
	if vmfs, ok := ds.Info.(*types.VmfsDatastoreInfo); ok {
		volumeUUID = vmfs.Vmfs.Uuid
	}

	for _, mount := range ds.Host {
		hf, err := facts.forHost(ctx, pc, mount.Key)
		if err != nil {
			continue // this host's storage view is unavailable; try the next
		}
		volume := hf.vmfsVolume(volumeUUID, ds.Summary.Name)
		if volume == nil {
			continue
		}
		if t := TransportForExtents(volume.Extent, hf.lunKeyByCanonicalName, hf.adapterKeyByLunKey, hf.descriptorByAdapterKey); t != "" {
			return t
		}
	}
	return TransportUnknown
}

// storageFactCache memoizes per-host storage topology across datastores.
type storageFactCache struct {
	byHost map[types.ManagedObjectReference]*hostStorageFacts
}

func newStorageFactCache() *storageFactCache {
	return &storageFactCache{byHost: map[types.ManagedObjectReference]*hostStorageFacts{}}
}

// forHost fetches (and caches) the storage facts of one host.
func (fc *storageFactCache) forHost(ctx context.Context, pc *property.Collector, host types.ManagedObjectReference) (*hostStorageFacts, error) {
	if f, ok := fc.byHost[host]; ok {
		return f, nil
	}

	var hostObj mo.HostSystem
	if err := pc.RetrieveOne(ctx, host, []string{"configManager.storageSystem"}, &hostObj); err != nil {
		return nil, fmt.Errorf("retrieve host %s config manager: %w", host, err)
	}
	ssRef := hostObj.ConfigManager.StorageSystem
	if ssRef == nil || ssRef.Type == "" || ssRef.Value == "" {
		return nil, fmt.Errorf("host %s has no storage system", host)
	}

	var hss mo.HostStorageSystem
	if err := pc.RetrieveOne(ctx, *ssRef,
		[]string{"fileSystemVolumeInfo", "storageDeviceInfo"}, &hss); err != nil {
		return nil, fmt.Errorf("retrieve host %s storage system: %w", host, err)
	}

	f := &hostStorageFacts{
		vmfsVolumes:            map[string]*types.HostVmfsVolume{},
		lunKeyByCanonicalName:  map[string]string{},
		adapterKeyByLunKey:     map[string]string{},
		descriptorByAdapterKey: map[string]string{},
	}
	for _, m := range hss.FileSystemVolumeInfo.MountInfo {
		if vmfs, ok := m.Volume.(*types.HostVmfsVolume); ok {
			f.vmfsVolumes[vmfs.Uuid] = vmfs
			f.vmfsVolumes[vmfs.Name] = vmfs
		}
	}
	if dev := hss.StorageDeviceInfo; dev != nil {
		for _, b := range dev.ScsiLun {
			lun := b.GetScsiLun()
			f.lunKeyByCanonicalName[lun.CanonicalName] = lun.Key
		}
		for _, b := range dev.HostBusAdapter {
			hba := b.GetHostHostBusAdapter()
			f.descriptorByAdapterKey[hba.Key] = HBADescriptor(b)
		}
		if top := dev.ScsiTopology; top != nil {
			for _, iface := range top.Adapter {
				for _, target := range iface.Target {
					for _, lun := range target.Lun {
						f.adapterKeyByLunKey[lun.ScsiLun] = iface.Adapter
					}
				}
			}
		}
	}

	fc.byHost[host] = f
	return f, nil
}

// hostStorageFacts is one host's denormalized storage topology.
type hostStorageFacts struct {
	vmfsVolumes            map[string]*types.HostVmfsVolume // by uuid and by name
	lunKeyByCanonicalName  map[string]string
	adapterKeyByLunKey     map[string]string
	descriptorByAdapterKey map[string]string
}

func (f *hostStorageFacts) vmfsVolume(uuid, name string) *types.HostVmfsVolume {
	if uuid != "" {
		if v, ok := f.vmfsVolumes[uuid]; ok {
			return v
		}
	}
	return f.vmfsVolumes[name]
}

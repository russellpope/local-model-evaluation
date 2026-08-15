package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// DatastoreInfo is one row of the datastores table.
type DatastoreInfo struct {
	Name           string
	Type           string // FC, iSCSI, NVMe, NFS, or unknown
	CapacityBytes  int64
	AvailableBytes int64
	UsedBytes      int64
}

func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	refs, err := viewRefs(ctx, c, []string{"Datastore"})
	if err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}
	if len(refs) == 0 {
		return nil, nil
	}

	var objs []mo.Datastore
	if err := property.DefaultCollector(c).Retrieve(ctx, refs,
		[]string{"name", "summary", "info"}, &objs); err != nil {
		return nil, fmt.Errorf("collect datastore properties: %w", err)
	}

	hosts, err := hostStorages(ctx, c)
	if err != nil {
		return nil, err
	}

	infos := make([]DatastoreInfo, 0, len(objs))
	for i := range objs {
		ds := &objs[i]
		used := UsedBytes(ds.Summary.Capacity, ds.Summary.FreeSpace)
		infos = append(infos, DatastoreInfo{
			Name:           ds.Name,
			Type:           datastoreTransport(hosts, ds),
			CapacityBytes:  ds.Summary.Capacity,
			AvailableBytes: ds.Summary.FreeSpace,
			UsedBytes:      used,
		})
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, nil
}

// hostStorage is the per-host view needed to map a datastore to the transport
// of its backing storage devices: VMFS volumes (name, uuid, backing disk
// names), SCSI disks (by canonical name and LUN key), the LUN-to-adapter
// topology, and the adapters themselves.
type hostStorage struct {
	diskByCanonical map[string]*types.HostScsiDisk
	lunToAdapter    map[string]string
	hbaByName       map[string]HBADescriptor
	volumes         []hostVolume
}

type hostVolume struct {
	name    string
	uuid    string
	extents []string
}

func hostStorages(ctx context.Context, c *vim25.Client) ([]*hostStorage, error) {
	refs, err := viewRefs(ctx, c, []string{"HostSystem"})
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	if len(refs) == 0 {
		return nil, nil
	}
	var objs []mo.HostSystem
	if err := property.DefaultCollector(c).Retrieve(ctx, refs,
		[]string{"name", "config.storageDevice", "config.fileSystemVolume"}, &objs); err != nil {
		return nil, fmt.Errorf("collect host storage properties: %w", err)
	}
	storages := make([]*hostStorage, 0, len(objs))
	for i := range objs {
		storages = append(storages, newHostStorage(&objs[i]))
	}
	return storages, nil
}

func newHostStorage(host *mo.HostSystem) *hostStorage {
	hs := &hostStorage{
		diskByCanonical: map[string]*types.HostScsiDisk{},
		lunToAdapter:    map[string]string{},
		hbaByName:       map[string]HBADescriptor{},
	}
	cfg := host.Config
	if cfg == nil {
		return hs
	}

	sd := cfg.StorageDevice
	if sd != nil {
		for i := range sd.ScsiLun {
			if disk, ok := sd.ScsiLun[i].(*types.HostScsiDisk); ok {
				hs.diskByCanonical[disk.CanonicalName] = disk
				hs.lunToAdapter[disk.ScsiLun.Key] = ""
			}
		}
		if sd.ScsiTopology != nil {
			for _, iface := range sd.ScsiTopology.Adapter {
				for _, target := range iface.Target {
					for _, lun := range target.Lun {
						if _, ok := hs.lunToAdapter[lun.ScsiLun]; ok {
							hs.lunToAdapter[lun.ScsiLun] = iface.Adapter
						}
					}
				}
			}
		}
		for _, hba := range sd.HostBusAdapter {
			base := hba.GetHostHostBusAdapter()
			if base == nil {
				continue
			}
			desc := hbaDescriptor(hba, base)
			if base.Device != "" {
				hs.hbaByName[base.Device] = desc
			}
			if base.Key != "" {
				hs.hbaByName[base.Key] = desc
			}
		}
	}

	if fsv := cfg.FileSystemVolume; fsv != nil {
		for _, mount := range fsv.MountInfo {
			vmfs, ok := mount.Volume.(*types.HostVmfsVolume)
			if !ok {
				continue
			}
			vol := hostVolume{name: vmfs.Name, uuid: vmfs.Uuid}
			for _, extent := range vmfs.Extent {
				vol.extents = append(vol.extents, extent.DiskName)
			}
			hs.volumes = append(hs.volumes, vol)
		}
	}

	return hs
}

func hbaDescriptor(hba types.BaseHostHostBusAdapter, base *types.HostHostBusAdapter) HBADescriptor {
	desc := HBADescriptor{
		StorageProtocol: base.StorageProtocol,
		Kind:            "other",
		Model:           base.Model,
		Driver:          base.Driver,
	}
	switch hba.(type) {
	case *types.HostFibreChannelHba:
		desc.Kind = "fibrechannel"
	case *types.HostFibreChannelOverEthernetHba:
		desc.Kind = "fibrechannelethernet"
	case *types.HostInternetScsiHba:
		desc.Kind = "internetscsi"
	case *types.HostBlockHba:
		desc.Kind = "block"
	case *types.HostParallelScsiHba:
		desc.Kind = "parallelscsi"
	case *types.HostSerialAttachedHba:
		desc.Kind = "serialattached"
	case *types.HostTcpHba:
		desc.Kind = "tcp"
	case *types.HostRdmaHba:
		desc.Kind = "rdma"
	case *types.HostPcieHba:
		desc.Kind = "pcie"
	}
	return desc
}

// datastoreTransport derives the transport (FC/iSCSI/NVMe/NFS) of a
// datastore from its backing storage devices on the hosts that mount it.
// NFS datastores report NFS. When the backing device cannot be resolved to
// a known transport (e.g. against vcsim, which does not model storage
// topology), the result degrades to "unknown".
func datastoreTransport(hosts []*hostStorage, ds *mo.Datastore) string {
	if strings.EqualFold(ds.Summary.Type, "nfs") {
		return TransportNFS
	}
	uuid := vmfsUUID(ds)
	for _, hs := range hosts {
		for _, vol := range hs.volumes {
			if vol.name != ds.Name && (uuid == "" || vol.uuid != uuid) {
				continue
			}
			for _, diskName := range vol.extents {
				disk, ok := hs.diskByCanonical[diskName]
				if !ok {
					continue
				}
				adapter := hs.lunToAdapter[disk.ScsiLun.Key]
				hba, ok := hs.hbaByName[adapter]
				if !ok {
					continue
				}
				if t := ClassifyTransport(ds.Summary.Type, hba); t != TransportUnknown {
					return t
				}
			}
		}
	}
	return TransportUnknown
}

// vmfsUUID extracts the VMFS volume UUID from a datastore URL of the form
// vmfs/<uuid>. It returns "" when the datastore is not a VMFS path.
func vmfsUUID(ds *mo.Datastore) string {
	if ds.Info == nil {
		return ""
	}
	u := ds.Info.GetDatastoreInfo().Url
	const prefix = "vmfs/"
	if i := strings.Index(u, prefix); i >= 0 {
		rest := u[i+len(prefix):]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	return ""
}

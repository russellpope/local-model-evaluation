package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/local-model-evaluation/vsphere-inventory/internal/format"
)

// DatastoreInfo is one row of the datastores report.
type DatastoreInfo struct {
	Name string
	// Type is the storage transport: FC, iSCSI, NVMe, NFS or unknown.
	Type string
	// Capacity, Available and Used are in bytes. Used = Capacity - Available.
	Capacity  int64
	Available int64
	Used      int64
}

// hostStorage is the per-host storage topology used to derive datastores'
// backing transport.
type hostStorage struct {
	// hbas maps an HBA key (e.g. "vmhba1") to its descriptor.
	hbas map[string]HbaInfo
	// luns are the LUN-to-path observations on the host.
	luns []LunPath
	// volumeExtents maps a VMFS volume uuid to the device ids (canonical
	// names) of its extents.
	volumeExtents map[string][]string
}

// ListDatastores returns every datastore in the inventory, sorted by name.
func ListDatastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	refs, err := findRefs(ctx, c, "Datastore")
	if err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}

	var content []mo.Datastore
	if err := retrieve(ctx, c, refs, []string{"name", "info", "summary", "host"}, &content); err != nil {
		return nil, err
	}

	storage, err := loadHostStorage(ctx, c, content)
	if err != nil {
		return nil, err
	}

	out := make([]DatastoreInfo, 0, len(content))
	for _, m := range content {
		info := DatastoreInfo{
			Name:      m.Name,
			Capacity:  m.Summary.Capacity,
			Available: m.Summary.FreeSpace,
		}
		info.Used = format.UsedBytes(info.Capacity, info.Available)
		info.Type = resolveTransport(m, storage)
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// loadHostStorage fetches the storage topology (HBAs, LUN paths, VMFS
// volume extents) of every host that mounts one of the given datastores.
func loadHostStorage(ctx context.Context, c *vim25.Client, dss []mo.Datastore) (map[string]*hostStorage, error) {
	seen := make(map[string]bool)
	hostRefs := make([]types.ManagedObjectReference, 0)
	for _, ds := range dss {
		for _, mount := range ds.Host {
			if seen[mount.Key.Value] {
				continue
			}
			seen[mount.Key.Value] = true
			hostRefs = append(hostRefs, mount.Key)
		}
	}

	var hosts []mo.HostSystem
	if err := retrieve(ctx, c, hostRefs, []string{"configManager.storageSystem"}, &hosts); err != nil {
		return nil, fmt.Errorf("load host storage systems: %w", err)
	}

	byHost := make(map[string]*hostStorage, len(hosts))
	hostBySS := make(map[string]string, len(hosts))
	ssSeen := make(map[string]bool)
	ssRefs := make([]types.ManagedObjectReference, 0, len(hosts))
	for _, h := range hosts {
		host := &hostStorage{
			hbas:          make(map[string]HbaInfo),
			volumeExtents: make(map[string][]string),
		}
		byHost[h.Self.Value] = host

		if h.ConfigManager.StorageSystem == nil {
			continue
		}
		ref := *h.ConfigManager.StorageSystem
		if ssSeen[ref.Value] {
			continue
		}
		ssSeen[ref.Value] = true
		hostBySS[ref.Value] = h.Self.Value
		ssRefs = append(ssRefs, ref)
	}

	var systems []mo.HostStorageSystem
	if err := retrieve(ctx, c, ssRefs, []string{"storageDeviceInfo", "fileSystemVolumeInfo"}, &systems); err != nil {
		return nil, fmt.Errorf("load storage system properties: %w", err)
	}

	for _, s := range systems {
		host := byHost[hostBySS[s.Self.Value]]
		if host == nil {
			continue
		}
		mergeHostStorage(host, s)
	}

	return byHost, nil
}

func mergeHostStorage(dst *hostStorage, s mo.HostStorageSystem) {
	if s.StorageDeviceInfo != nil {
		for _, hba := range s.StorageDeviceInfo.HostBusAdapter {
			base := hba.GetHostHostBusAdapter()
			key := base.Key
			if key == "" {
				key = base.Device
			}
			if key == "" {
				continue
			}
			dst.hbas[key] = HbaInfo{
				Key:             key,
				Type:            hbaTypeOf(hba),
				StorageProtocol: base.StorageProtocol,
			}
		}
		if s.StorageDeviceInfo.MultipathInfo != nil {
			for _, lun := range s.StorageDeviceInfo.MultipathInfo.Lun {
				for _, p := range lun.Path {
					dst.luns = append(dst.luns, LunPath{Device: lun.Id, Path: p.Name})
				}
			}
		}
	}

	for _, mount := range s.FileSystemVolumeInfo.MountInfo {
		vol, ok := mount.Volume.(*types.HostVmfsVolume)
		if !ok || vol.Uuid == "" {
			continue
		}
		for _, extent := range vol.Extent {
			if extent.DiskName == "" {
				continue
			}
			dst.volumeExtents[vol.Uuid] = append(dst.volumeExtents[vol.Uuid], extent.DiskName)
		}
	}
}

func hbaTypeOf(hba types.BaseHostHostBusAdapter) string {
	switch hba.(type) {
	case *types.HostFibreChannelHba:
		return "HostFibreChannelHba"
	case *types.HostInternetScsiHba:
		return "HostInternetScsiHba"
	case *types.HostPcieHba:
		return "HostPcieHba"
	case *types.HostSerialAttachedHba:
		return "HostSerialAttachedHba"
	default:
		return ""
	}
}

// resolveTransport derives the storage transport of a datastore from its
// backing storage devices and host bus adapters. NFS datastores report
// NFS; everything that cannot be derived from the API reports unknown.
func resolveTransport(ds mo.Datastore, storage map[string]*hostStorage) string {
	if strings.EqualFold(ds.Summary.Type, "nfs") {
		return TransportNFS
	}

	uuid := datastoreVolumeUUID(ds.Info.GetDatastoreInfo().Url)
	if uuid == "" {
		return TransportUnknown
	}

	var devices []string
	var luns []LunPath
	hbas := make(map[string]HbaInfo)
	seenHost := make(map[string]bool)

	for _, mount := range ds.Host {
		if seenHost[mount.Key.Value] {
			continue
		}
		seenHost[mount.Key.Value] = true

		st := storage[mount.Key.Value]
		if st == nil {
			continue
		}
		for _, d := range st.volumeExtents[uuid] {
			devices = append(devices, d)
		}
		luns = append(luns, st.luns...)
		for k, v := range st.hbas {
			hbas[k] = v
		}
	}

	if len(devices) == 0 {
		return TransportUnknown
	}

	return ClassifyTransport(devices, luns, hbas)
}

// datastoreVolumeUUID extracts the VMFS volume uuid from a datastore URL of
// the form "ds:///vmfs/volumes/<uuid>/...". It returns "" for URLs that do
// not carry a volume uuid (local file datastores, NAS paths, ...).
func datastoreVolumeUUID(url string) string {
	const prefix = "ds:///vmfs/volumes/"
	if !strings.HasPrefix(url, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(url, prefix)
	seg := rest
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		seg = rest[:i]
	}
	return seg
}

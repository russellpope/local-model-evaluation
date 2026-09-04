package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"vsphere-inventory/internal/format"
)

// DatastoreInfo describes a datastore: its name, the transport backing it
// (FC/iSCSI/NVMe/NFS or unknown), and its capacity split into used and
// available bytes (used = capacity - available).
type DatastoreInfo struct {
	Name           string
	Transport      string
	CapacityBytes  int64
	UsedBytes      int64
	AvailableBytes int64
}

// ListDatastores returns all datastores in the inventory, sorted by name.
//
// Transport derivation: NFS datastores report NFS. VMFS datastores are
// classified by looking at the physical backing of their extents: the
// datastore's extents name SCSI disks (by canonical name); the disks are
// joined with the host's multipath configuration to find the host bus
// adapters providing paths to each disk, and the HBA types (fibre channel,
// iSCSI, NVMe) determine the transport. When the API does not expose
// enough topology (for example local datastores or the simulator), the
// transport degrades to "unknown".
func ListDatastores(ctx context.Context, c *govmomi.Client) ([]DatastoreInfo, error) {
	v, err := newContainerView(ctx, c, []string{"Datastore", "HostSystem"})
	if err != nil {
		return nil, err
	}
	defer v.Destroy(ctx)

	var dss []mo.Datastore
	err = v.Retrieve(ctx, []string{"Datastore"}, []string{"name", "info", "summary", "host"}, &dss)
	if err != nil {
		return nil, fmt.Errorf("retrieve datastores: %w", err)
	}

	var hosts []mo.HostSystem
	err = v.Retrieve(ctx, []string{"HostSystem"}, []string{"name", "configManager.storageSystem"}, &hosts)
	if err != nil {
		return nil, fmt.Errorf("retrieve hosts: %w", err)
	}

	// Fetch every host's storage device info in one call; transport
	// derivation is per (datastore, mounting host) and datastores share
	// hosts, so cache the results.
	storage := make(map[types.ManagedObjectReference]*types.HostStorageDeviceInfo, len(hosts))
	{
		var refs []types.ManagedObjectReference
		for _, h := range hosts {
			if h.ConfigManager.StorageSystem != nil {
				refs = append(refs, *h.ConfigManager.StorageSystem)
			}
		}
		if len(refs) > 0 {
			var systems []mo.HostStorageSystem
			pc := property.DefaultCollector(c.Client)
			if err := pc.Retrieve(ctx, refs, []string{"storageDeviceInfo"}, &systems); err != nil {
				return nil, fmt.Errorf("retrieve host storage device info: %w", err)
			}
			for _, s := range systems {
				storage[s.Reference()] = s.StorageDeviceInfo
			}
		}
	}

	hostRefs := make([]types.ManagedObjectReference, 0, len(hosts))
	for _, h := range hosts {
		hostRefs = append(hostRefs, h.Reference())
	}

	out := make([]DatastoreInfo, 0, len(dss))
	for _, ds := range dss {
		info := DatastoreInfo{
			Name:           ds.Name,
			CapacityBytes:  ds.Summary.Capacity,
			AvailableBytes: ds.Summary.FreeSpace,
			UsedBytes:      format.UsedBytes(int64(ds.Summary.Capacity), int64(ds.Summary.FreeSpace)),
		}
		info.Transport = deriveTransport(ds, hostRefs, storage)
		out = append(out, info)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// deriveTransport classifies one datastore. It is permissive by design: any
// gap in the topology data yields TransportUnknown instead of an error.
func deriveTransport(ds mo.Datastore, hosts []types.ManagedObjectReference, storage map[types.ManagedObjectReference]*types.HostStorageDeviceInfo) string {
	switch dsi := ds.Info.(type) {
	case *types.NasDatastoreInfo:
		return TransportNFS
	case *types.VmfsDatastoreInfo:
		if dsi.Vmfs == nil || len(dsi.Vmfs.Extent) == 0 {
			return TransportUnknown
		}
		return classifyVmfs(dsi.Vmfs.Extent, ds.Host, hosts, storage)
	default:
		// Local datastores, vSAN, vVol, or anything else without a
		// block-transport topology.
		return TransportUnknown
	}
}

// classifyVmfs derives the transport for a VMFS volume from its extents.
func classifyVmfs(
	extents []types.HostScsiDiskPartition,
	mounts []types.DatastoreHostMount,
	hosts []types.ManagedObjectReference,
	storage map[types.ManagedObjectReference]*types.HostStorageDeviceInfo,
) string {
	for _, mount := range mounts {
		info := storage[mount.Key]
		if info == nil {
			continue
		}
		if t := classifyVmfsFromHost(extents, info); t != TransportUnknown {
			return t
		}
	}
	// No mounting host exposed usable storage topology.
	return TransportUnknown
}

// classifyVmfsFromHost classifies extents using one host's storage topology.
func classifyVmfsFromHost(extents []types.HostScsiDiskPartition, info *types.HostStorageDeviceInfo) string {
	// Join disk canonical name -> multipath adapters via the ScsiLun key.
	adapterTypesByCanonical := make(map[string][]string)
	for _, baseLun := range info.ScsiLun {
		lun, ok := baseLun.(*types.HostScsiDisk)
		if !ok || lun.CanonicalName == "" {
			continue
		}
		var adapters []string
		if info.MultipathInfo != nil {
			for _, mpLun := range info.MultipathInfo.Lun {
				if mpLun.Lun != lun.Key && mpLun.Lun != lun.CanonicalName {
					continue
				}
				for _, p := range mpLun.Path {
					if t := adapterType(p.Adapter); t != "" {
						adapters = append(adapters, t)
					}
				}
			}
		}
		adapterTypesByCanonical[lun.CanonicalName] = append(adapterTypesByCanonical[lun.CanonicalName], adapters...)

		descriptor := LunDescriptor{
			CanonicalName: lun.CanonicalName,
			DeviceName:    lun.DeviceName,
			DeviceType:    lun.DeviceType,
			Vendor:        lun.Vendor,
			Model:         lun.Model,
			AdapterTypes:  adapterTypesByCanonical[lun.CanonicalName],
		}
		for _, ext := range extents {
			if ext.DiskName == lun.CanonicalName {
				if t := ClassifyTransport(descriptor); t != TransportUnknown {
					return t
				}
			}
		}
	}

	// NVMe-over-fabrics namespaces do not appear as HostScsiDisk entries on
	// the ScsiLun list; check the NVMe topology namespaces directly.
	if info.NvmeTopology != nil {
		names := nvmeNamespaceNames(info.NvmeTopology)
		for _, ext := range extents {
			if names[ext.DiskName] {
				return TransportNVMe
			}
		}
	}

	return TransportUnknown
}

// nvmeNamespaceNames collects the device names of all namespaces attached
// through the host's NVMe controllers.
func nvmeNamespaceNames(topo *types.HostNvmeTopology) map[string]bool {
	names := make(map[string]bool)
	for _, iface := range topo.Adapter {
		for _, ctrl := range iface.ConnectedController {
			for _, ns := range ctrl.AttachedNamespace {
				names[ns.Name] = true
			}
		}
	}
	return names
}

// adapterType decodes an HBA key string of the form
// "key-vim.host.FibreChannelHba-vmhba1" into its type component.
func adapterType(adapterKey string) string {
	key := strings.TrimPrefix(adapterKey, "key-vim.host.")
	if i := strings.Index(key, "-"); i > 0 {
		return key[:i]
	}
	return key
}

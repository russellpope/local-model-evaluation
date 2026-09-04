package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	"github.com/ldh/vsphere-inventory/internal/format"
)

// Transport (protocol) classification constants.
const (
	TransportFC      = "FC"
	TransportiSCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// Datastores returns all datastores sorted by name. Transport is the actual
// storage protocol (FC/iSCSI/NVMe/NFS), derived from the datastore's backing
// extents on its mounting hosts — not the filesystem type. A VMFS datastore
// may be backed by FC, iSCSI, or NVMe; NFS datastores report NFS. When no
// backing can be resolved the transport degrades to "unknown".
func Datastores(ctx context.Context, c *vim25.Client) ([]DatastoreInfo, error) {
	var dss []mo.Datastore
	if err := retrieveAll(ctx, c, "Datastore", []string{"name", "summary", "host"}, &dss); err != nil {
		return nil, fmt.Errorf("list datastores: %w", err)
	}
	var hosts []mo.HostSystem
	if err := retrieveAll(ctx, c, "HostSystem",
		[]string{"name", "config.storageDevice", "config.fileSystemVolume"}, &hosts); err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}

	// diskName (extent / namespace) -> protocol, aggregated across hosts.
	diskProto := map[string]string{}
	for i := range hosts {
		if hosts[i].Config == nil {
			continue
		}
		for k, v := range hostDiskProtocols(hosts[i].Config.StorageDevice) {
			if _, ok := diskProto[k]; !ok {
				diskProto[k] = v
			}
		}
	}

	// vmfsVolumeName -> protocol, via extents on mounted VMFS volumes.
	volProto := map[string]string{}
	for i := range hosts {
		cfg := hosts[i].Config
		if cfg == nil || cfg.FileSystemVolume == nil {
			continue
		}
		for _, m := range cfg.FileSystemVolume.MountInfo {
			if m.MountInfo.Mounted == nil || !*m.MountInfo.Mounted {
				continue
			}
			vmfs, ok := m.Volume.(*types.HostVmfsVolume)
			if !ok {
				continue
			}
			name := vmfs.Name
			if name == "" || volProto[name] != "" {
				continue
			}
			for _, ext := range vmfs.Extent {
				if p, ok := diskProto[ext.DiskName]; ok && p != TransportUnknown {
					volProto[name] = p
					break
				}
			}
		}
	}

	out := make([]DatastoreInfo, 0, len(dss))
	for _, ds := range dss {
		info := DatastoreInfo{Name: ds.Name, Transport: TransportUnknown}
		info.FsType = ds.Summary.Type
		info.Capacity = ds.Summary.Capacity
		info.Available = ds.Summary.FreeSpace
		info.Used = format.Used(ds.Summary.Capacity, ds.Summary.FreeSpace)
		if isNfsFsType(ds.Summary.Type) {
			info.Transport = TransportNFS
		} else if p, ok := volProto[ds.Name]; ok {
			info.Transport = p
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func isNfsFsType(t string) bool {
	u := strings.ToUpper(strings.TrimSpace(t))
	return u == "NFS" || u == "NFS4" || u == "NFS41"
}

// hostDiskProtocols maps every backing disk or NVMe namespace name known to
// one host to its storage protocol, using HBAs, SCSI topology, multipath
// paths, and the NVMe topology.
func hostDiskProtocols(sd *types.HostStorageDeviceInfo) map[string]string {
	out := map[string]string{}
	if sd == nil {
		return out
	}

	// 1. Classify each HBA by its concrete API type and driver.
	adapterProto := map[string]string{}
	for _, hba := range sd.HostBusAdapter {
		d := hba.GetHostHostBusAdapter()
		kind := hbaKind(hba)
		if sp := strings.ToLower(d.StorageProtocol); sp != "" && sp != "scsi" {
			kind = sp
		}
		desc := DeviceDescriptor{Device: d.Device, Driver: d.Driver, Model: d.Model, Kind: kind}
		switch h := hba.(type) {
		case *types.HostFibreChannelHba:
			desc.WWN = fmt.Sprintf("%016x", h.PortWorldWideName)
		case *types.HostInternetScsiHba:
			desc.IQN = h.IScsiName
		}
		if p := ClassifyTransport(desc); p != TransportUnknown {
			adapterProto[d.Device] = p
		}
	}

	// LUN key -> disk identifier names (canonical + descriptors).
	lunNames := map[string][]string{}
	for _, lun := range sd.ScsiLun {
		l := lun.GetScsiLun()
		var names []string
		if l.CanonicalName != "" {
			names = append(names, l.CanonicalName)
		}
		for _, dd := range l.Descriptor {
			if dd.Id != "" {
				names = append(names, dd.Id)
			}
		}
		if len(names) > 0 {
			lunNames[l.Key] = names
		}
	}

	// 2. SCSI topology: per-target transport refines the adapter protocol.
	if sd.ScsiTopology != nil {
		for _, iface := range sd.ScsiTopology.Adapter {
			for _, target := range iface.Target {
				p := ClassifyTransport(DeviceDescriptor{
					Device: iface.Adapter,
					Kind:   targetTransportKind(target.Transport),
				})
				if p == TransportUnknown {
					p = adapterProto[iface.Adapter]
				}
				if p == TransportUnknown {
					continue
				}
				for _, tl := range target.Lun {
					for _, name := range append(lunNames[tl.ScsiLun], tl.Key) {
						out[name] = p
					}
				}
			}
		}
	}

	// 3. Multipath: LUN inherits the protocol of its first known adapter.
	if sd.MultipathInfo != nil {
		for _, lu := range sd.MultipathInfo.Lun {
			p := TransportUnknown
			for _, path := range lu.Path {
				if ap := adapterProto[path.Adapter]; ap != TransportUnknown {
					p = ap
					break
				}
			}
			if p == TransportUnknown {
				continue
			}
			names := append([]string{lu.Id}, lunNames[lu.Lun]...)
			for _, name := range names {
				if name != "" {
					out[name] = p
				}
			}
		}
	}

	// 4. NVMe namespaces.
	if sd.NvmeTopology != nil {
		for _, iface := range sd.NvmeTopology.Adapter {
			for _, ctrl := range iface.ConnectedController {
				p := adapterProto[iface.Adapter]
				if p == TransportUnknown {
					p = ClassifyTransport(DeviceDescriptor{
						Device: ctrl.AssociatedAdapter,
						Kind:   strings.ToLower(ctrl.TransportType),
					})
				}
				if p == TransportUnknown {
					continue
				}
				for _, ns := range ctrl.AttachedNamespace {
					if ns.Name != "" {
						out[ns.Name] = p
					}
				}
			}
		}
	}
	return out
}

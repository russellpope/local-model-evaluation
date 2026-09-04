package inventory

import (
	"fmt"
	"strings"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Storage transport identifiers reported by the datastores subcommand.
const (
	TransportFC      = "FC"
	TransportISCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// ClassifyTransport maps a storage-device descriptor — a concrete HBA type
// name (e.g. "types.HostFibreChannelHba"), a driver name, or a namespace
// transport — to the underlying transport protocol. Unrecognized inputs
// degrade to "unknown" rather than guessing.
func ClassifyTransport(descriptor string) string {
	d := strings.ToLower(descriptor)
	switch {
	case strings.Contains(d, "fibrechannel"),
		strings.Contains(d, "fcp"),
		strings.Contains(d, "lpfc"),    // common FC driver
		strings.Contains(d, "qla2xxx"): // FC mode of the QLogic driver
		return TransportFC
	case strings.Contains(d, "iscsi"),
		strings.Contains(d, "internetscsi"): // HostInternetScsiHba carries "netScsi", not "iscsi"
		return TransportISCSI
	case strings.Contains(d, "nvme"):
		return TransportNVMe
	case strings.Contains(d, "nfs"):
		return TransportNFS
	default:
		return TransportUnknown
	}
}

// datastoreExtents returns backing device names (e.g. "naa.600...") for a VMFS
// datastore, preferring the datastore object's own info and falling back to
// the per-host mount table.
func datastoreExtents(ds *mo.Datastore, hosts []mo.HostSystem) []string {
	if info, ok := ds.Info.(*types.VmfsDatastoreInfo); ok && info != nil && info.Vmfs != nil {
		names := make([]string, 0, len(info.Vmfs.Extent))
		for _, ext := range info.Vmfs.Extent {
			if ext.DiskName != "" {
				names = append(names, ext.DiskName)
			}
		}
		if len(names) > 0 {
			return names
		}
	}
	// Fall back to the host-side mount table (volumes are named after the
	// datastore, extents are only visible per host).
	mounted := make(map[string]bool)
	var names []string
	for _, h := range hosts {
		if h.Config == nil || h.Config.FileSystemVolume == nil {
			continue
		}
		for _, mi := range h.Config.FileSystemVolume.MountInfo {
			vol := mi.Volume
			if vol == nil || vol.GetHostFileSystemVolume().Name != ds.Summary.Name {
				continue
			}
			if vmfs, ok := vol.(*types.HostVmfsVolume); ok {
				for _, ext := range vmfs.Extent {
					if ext.DiskName != "" && !mounted[ext.DiskName] {
						mounted[ext.DiskName] = true
						names = append(names, ext.DiskName)
					}
				}
			}
		}
	}
	return names
}

// resolveDatastoreTransport derives the actual transport (FC/iSCSI/NVMe/NFS)
// backing a datastore from its extents and the storage devices of the hosts
// mounting it. The datastore's filesystem type is deliberately not consulted
// (a VMFS volume may ride FC, iSCSI or NVMe); only NFS is reported from the
// filesystem type because NFS *is* the transport there. Anything that cannot
// be derived degrades to "unknown".
func isNFSType(fsType string) bool {
	return strings.HasPrefix(strings.ToUpper(fsType), "NFS")
}

func resolveDatastoreTransport(ds *mo.Datastore, hosts []mo.HostSystem) string {
	if isNFSType(ds.Summary.Type) {
		return TransportNFS
	}

	mounting := mountingHosts(ds, hosts)
	extents := datastoreExtents(ds, mounting)
	for _, diskName := range extents {
		for i := range mounting {
			if t := classifyExtent(&mounting[i], diskName); t != TransportUnknown {
				return t
			}
		}
	}
	return TransportUnknown
}

// mountingHosts returns the hosts that mount the datastore.
func mountingHosts(ds *mo.Datastore, hosts []mo.HostSystem) []mo.HostSystem {
	if len(ds.Host) == 0 {
		return hosts
	}
	want := make(map[string]bool, len(ds.Host))
	for _, hm := range ds.Host {
		want[hm.Key.Value] = true
	}
	var out []mo.HostSystem
	for _, h := range hosts {
		if want[h.Self.Value] {
			out = append(out, h)
		}
	}
	return out
}

// classifyExtent maps one backing device name to a transport using one host's
// storage topology.
func classifyExtent(host *mo.HostSystem, diskName string) string {
	if host.Config == nil {
		return TransportUnknown
	}
	sd := host.Config.StorageDevice
	if sd == nil {
		return TransportUnknown
	}

	// NVMe: the extent matches a namespace attached to an NVMe controller.
	if sd.NvmeTopology != nil {
		for _, ifc := range sd.NvmeTopology.Adapter {
			for _, ctrl := range ifc.ConnectedController {
				for _, ns := range ctrl.AttachedNamespace {
					if ns.Key == diskName || ns.Name == diskName ||
						strings.HasPrefix(ns.Key, diskName+":") {
						return TransportNVMe
					}
				}
			}
		}
	}

	// SCSI: find the LUN matching the extent, walk the topology to its
	// adapter, then classify the adapter's concrete HBA type.
	lunKey, ok := findScsiLunKey(sd, diskName)
	if !ok {
		return TransportUnknown
	}
	adapterDevice := scsiAdapterForLun(sd, lunKey)
	if adapterDevice == "" {
		return TransportUnknown
	}
	for _, hba := range sd.HostBusAdapter {
		if hba.GetHostHostBusAdapter().Device != adapterDevice {
			continue
		}
		return ClassifyTransport(fmt.Sprintf("%T", hba))
	}
	return TransportUnknown
}

func findScsiLunKey(sd *types.HostStorageDeviceInfo, diskName string) (string, bool) {
	for _, lun := range sd.ScsiLun {
		l := lun.GetScsiLun()
		if l == nil {
			continue
		}
		if l.CanonicalName == diskName {
			return l.Key, true
		}
		for _, d := range l.Descriptor {
			if d.Id == diskName {
				return l.Key, true
			}
		}
	}
	return "", false
}

func scsiAdapterForLun(sd *types.HostStorageDeviceInfo, lunKey string) string {
	if sd.ScsiTopology == nil {
		return ""
	}
	for _, ifc := range sd.ScsiTopology.Adapter {
		for _, target := range ifc.Target {
			for _, lun := range target.Lun {
				if lun.ScsiLun == lunKey {
					return ifc.Adapter
				}
			}
		}
	}
	return ""
}

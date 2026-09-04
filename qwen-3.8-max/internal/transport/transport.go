package transport

import (
	"github.com/vmware/govmomi/vim25/types"
)

const (
	FC      = "FC"
	ISCSI   = "iSCSI"
	NVMe    = "NVMe"
	NFS     = "NFS"
	Unknown = "unknown"
)

func FromTargetTransport(t types.BaseHostTargetTransport) string {
	switch t.(type) {
	case *types.HostFibreChannelTargetTransport:
		return FC
	case *types.HostInternetScsiTargetTransport:
		return ISCSI
	default:
		return Unknown
	}
}

func FromHBA(h types.BaseHostHostBusAdapter) string {
	switch h.(type) {
	case *types.HostFibreChannelHba:
		return FC
	case *types.HostInternetScsiHba:
		return ISCSI
	default:
		return Unknown
	}
}

func DeviceInNVMeTopology(device string, topo *types.HostNvmeTopology) bool {
	if topo == nil || device == "" {
		return false
	}
	for _, adapter := range topo.Adapter {
		for _, controller := range adapter.ConnectedController {
			for _, ns := range controller.AttachedNamespace {
				if ns.Name == device {
					return true
				}
			}
		}
	}
	return false
}

func ClassifyDatastore(filesystemType string, extentDevices []string, info *types.HostStorageDeviceInfo) string {
	if filesystemType == "NFS" {
		return NFS
	}
	if info == nil {
		return Unknown
	}
	for _, device := range extentDevices {
		if DeviceInNVMeTopology(device, info.NvmeTopology) {
			return NVMe
		}
	}
	for _, device := range extentDevices {
		if t := scsiTransportForDevice(device, info); t != Unknown {
			return t
		}
	}
	return Unknown
}

func scsiTransportForDevice(device string, info *types.HostStorageDeviceInfo) string {
	lunKey := ""
	for _, lun := range info.ScsiLun {
		l := lun.GetScsiLun()
		if l == nil {
			continue
		}
		if l.DeviceName == device || l.CanonicalName == device {
			lunKey = l.Key
			break
		}
	}
	if lunKey == "" || info.ScsiTopology == nil {
		return Unknown
	}
	for _, adapter := range info.ScsiTopology.Adapter {
		for _, target := range adapter.Target {
			for _, lun := range target.Lun {
				if lun.ScsiLun == lunKey {
					return FromTargetTransport(target.Transport)
				}
			}
		}
	}
	return Unknown
}

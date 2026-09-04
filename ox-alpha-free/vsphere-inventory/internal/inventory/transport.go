package inventory

import (
	"github.com/vmware/govmomi/vim25/types"
)

// Kind tokens for storage paths, fed into ClassifyTransport.
const (
	KindFC    = "fc"
	KindiSCSI = "iscsi"
	KindNVMe  = "nvme"
)

// ClassifyTransport maps the set of path kinds through which a datastore's
// backing LUN(s) are reachable to a transport protocol: FC, iSCSI, NVMe, or
// unknown. A LUN is normally reached over exactly one fabric; if multiple
// fabrics are observed the result is deterministic with priority FC > iSCSI >
// NVMe. No recognized fabric yields "unknown" rather than a guess.
func ClassifyTransport(kinds []string) string {
	has := make(map[string]bool, len(kinds))
	for _, k := range kinds {
		has[k] = true
	}
	switch {
	case has[KindFC]:
		return "FC"
	case has[KindiSCSI]:
		return "iSCSI"
	case has[KindNVMe]:
		return "NVMe"
	default:
		return "unknown"
	}
}

// hbaKind maps an HBA to its fabric kind. FCoE counts as FC. PCIe HBAs carry
// local NVMe devices, HostTcpHba is defined as an NVMe-over-TCP adapter and
// HostRdmaHba carries NVMe/RDMA in vSphere, so all three map to NVMe.
func hbaKind(hba types.BaseHostHostBusAdapter) string {
	switch hba.(type) {
	case *types.HostFibreChannelHba, *types.HostFibreChannelOverEthernetHba:
		return KindFC
	case *types.HostInternetScsiHba:
		return KindiSCSI
	case *types.HostPcieHba, *types.HostTcpHba, *types.HostRdmaHba:
		return KindNVMe
	default:
		return ""
	}
}

// targetTransportKind maps a SCSI target transport to its fabric kind,
// mirroring hbaKind.
func targetTransportKind(t types.BaseHostTargetTransport) string {
	switch t.(type) {
	case *types.HostFibreChannelTargetTransport, *types.HostFibreChannelOverEthernetTargetTransport:
		return KindFC
	case *types.HostInternetScsiTargetTransport:
		return KindiSCSI
	case *types.HostPcieTargetTransport, *types.HostTcpTargetTransport:
		return KindNVMe
	default:
		return ""
	}
}

package inventory

import (
	"strings"

	"github.com/vmware/govmomi/vim25/types"
)

// DeviceDescriptor is a minimal, storage-subsystem-agnostic description of a
// host bus adapter or SCSI/NVMe target path. Factoring the FC/iSCSI/NVMe
// decision into ClassifyTransport over this struct is what makes the
// transport logic unit-testable: vcsim cannot model real storage transport
// topology (HBA -> LUN -> extent), so the correctness of the transport
// derivation is proven by the classifier's table test, not against the
// simulator.
type DeviceDescriptor struct {
	// Kind is a hint derived from the concrete API type: "fc", "fcoe",
	// "iscsi", "nvme", "sas", ... .
	Kind string
	// Device is the adapter or device identifier (e.g. "vmhba0").
	Device string
	// Driver is the kernel driver name (e.g. "lpfc", "bnx2i", "nvme").
	Driver string
	// Model is the adapter model string, when available.
	Model string
	// WWN is a fibre-channel world-wide name, when available.
	WWN string
	// IQN is an iSCSI qualified name, when available.
	IQN string
	// NAA is a disk identifier prefix, e.g. "naa.6000c29..." (SCSI) or
	// "eui.02004cf0..." (NVMe EUI-64).
	NAA string
}

// ClassifyTransport maps a device/HBA descriptor to a storage transport
// protocol: FC, iSCSI, NVMe, or unknown. Pure function; table-testable.
//
// Resolution order (most trusted signal first):
//  1. Explicit Kind from the concrete API type (HostFibreChannelHba, etc.).
//  2. Well-known ESXi driver names.
//  3. Address-format identifiers (IQN -> iSCSI, EUI/NQN -> NVMe,
//     WWN -> FC). naa.* alone is ambiguous (FC, iSCSI and SAS all use it),
//     so it never classifies on its own.
func ClassifyTransport(d DeviceDescriptor) string {
	switch kind := strings.ToLower(strings.TrimSpace(d.Kind)); kind {
	case "fc", "fibrechannel", "fibre_channel":
		return TransportFC
	case "fcoe":
		// FCoE carries FC frames over Ethernet; report FC.
		return TransportFC
	case "iscsi", "software_iscsi", "internet_scsi":
		return TransportiSCSI
	case "nvme", "nvme_tcp", "nvme_rdma", "nvme_fc", "nvmefc", "nvmrdma", "nvmetcp":
		return TransportNVMe
	case "sas", "sata", "usb", "scsi_bp", "blk":
		return TransportUnknown
	}

	switch driver := strings.ToLower(strings.TrimSpace(d.Driver)); driver {
	case "lpfc", "qla2xxx", "lpnic", "bfa", "ocs", "qedi", "il", "e4000nfc":
		return TransportFC
	case "bnx2i", "cxgb3i", "cxgb4i", "be2iscsi", "iscsi_tcp", "qla4xxx", "tcp":
		return TransportiSCSI
	case "nvme", "nvme_tcp", "nvme_rdma", "nvme_fc", "nvmefc", "vmknvme":
		return TransportNVMe
	}

	// Address-format fallbacks.
	if s := strings.ToLower(d.IQN); s != "" && (strings.HasPrefix(s, "iqn.") || strings.Contains(d.Device, "iqn.")) {
		return TransportiSCSI
	}
	if s := strings.ToLower(d.NAA); strings.HasPrefix(s, "eui.") || strings.HasPrefix(s, "nqn.") {
		return TransportNVMe
	}
	if strings.Contains(strings.ToLower(d.Device), "nqn.") {
		return TransportNVMe
	}
	if s := strings.ToLower(d.WWN); s != "" && s != "0" {
		// A populated world-wide name is FC addressing (iSCSI adapters
		// expose IQNs, NVMe exposes NQNs/EUIs).
		return TransportFC
	}
	return TransportUnknown
}

// hbaKind maps a concrete HostBusAdapter implementation to a Kind hint.
func hbaKind(hba types.BaseHostHostBusAdapter) string {
	switch hba.(type) {
	case *types.HostFibreChannelHba:
		return "fc"
	case *types.HostFibreChannelOverEthernetHba:
		return "fcoe"
	case *types.HostInternetScsiHba:
		return "iscsi"
	case *types.HostParallelScsiHba:
		return "sas"
	case *types.HostBlockHba:
		return "blk"
	default:
		return ""
	}
}

// targetTransportKind maps a SCSI target transport to a Kind hint.
func targetTransportKind(t types.BaseHostTargetTransport) string {
	switch t.(type) {
	case *types.HostFibreChannelTargetTransport:
		return "fc"
	case *types.HostInternetScsiTargetTransport:
		return "iscsi"
	case *types.HostBlockAdapterTargetTransport:
		return "nvme"
	default:
		return ""
	}
}

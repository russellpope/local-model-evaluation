package inventory

import (
	"github.com/vmware/govmomi/vim25/types"
)

// Transport values reported for datastores.
const (
	TransportFC      = "FC"
	TransportISCSCI  = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// HBADescriptor maps a concrete vSphere host bus adapter object to a stable
// descriptor string ("fibreChannel", "internetScsi", ...). The mapping from
// descriptor to transport lives in TransportFromHBADescriptor so it can be
// table-tested without a vCenter.
func HBADescriptor(hba types.BaseHostHostBusAdapter) string {
	switch hba.(type) {
	case *types.HostFibreChannelHba:
		return "fibreChannel"
	case *types.HostFibreChannelOverEthernetHba:
		return "fcoe"
	case *types.HostInternetScsiHba:
		return "internetScsi"
	case *types.HostPcieHba:
		// PCIe-attached NVMe devices surface as PCIe HBAs.
		return "pcie"
	case *types.HostTcpHba:
		// NVMe/TCP controllers surface as TCP HBAs.
		return "tcp"
	case *types.HostParallelScsiHba:
		return "parallelScsi"
	case *types.HostSerialAttachedHba:
		return "serialAttached"
	case *types.HostRdmaHba:
		return "rdma"
	case *types.HostBlockHba:
		return "block"
	default:
		return "unknown"
	}
}

// TransportFromHBADescriptor classifies a backing device/HBA descriptor into
// the storage transport that carries the datastore's extents. Descriptors
// that cannot be mapped to FC/iSCSI/NVMe (local SAS/SATA, IDE, RDMA, ...)
// degrade to "unknown" rather than guessing.
func TransportFromHBADescriptor(desc string) string {
	switch desc {
	case "fibreChannel", "fcoe":
		return TransportFC
	case "internetScsi":
		return TransportISCSCI
	case "pcie", "tcp":
		return TransportNVMe
	default:
		return TransportUnknown
	}
}

// TransportForExtents derives a VMFS datastore's transport from its backing
// devices: each extent's disk name is resolved to a SCSI LUN, the LUN to its
// topology adapter, and the adapter to an HBA whose descriptor classifies the
// transport. The first extent that yields a known transport wins; unmapped
// extents are skipped.
func TransportForExtents(
	extents []types.HostScsiDiskPartition,
	lunKeyByCanonicalName map[string]string,
	adapterKeyByLunKey map[string]string,
	descriptorByAdapterKey map[string]string,
) string {
	for _, ext := range extents {
		lunKey, ok := lunKeyByCanonicalName[ext.DiskName]
		if !ok {
			continue
		}
		adapterKey, ok := adapterKeyByLunKey[lunKey]
		if !ok {
			continue
		}
		desc, ok := descriptorByAdapterKey[adapterKey]
		if !ok {
			continue
		}
		if t := TransportFromHBADescriptor(desc); t != TransportUnknown {
			return t
		}
	}
	return ""
}

package inventory

import "strings"

// TransportDescriptor captures everything we know about a datastore's backing
// storage so that classifyTransport can decide on FC / iSCSI / NVMe / NFS /
// unknown without depending on govmomi types directly. The production code in
// datastores.go populates this struct from real API responses; the pure tests
// below construct it from representative inputs.
type TransportDescriptor struct {
	// FilesystemType is the datastore's filesystem type as reported by the API
	// (e.g. "VMFS", "NFS", "NFS41", "VSAN"). NFS-family types short-circuit to
	// "NFS" without looking at HBAs.
	FilesystemType string

	// HBAInfo lists the host bus adapters on a host that has this datastore's
	// LUN connected. Populated from HostHBA / HostFCPAdapter /
	// HostInternetScsiHba / HostNVMFController listings when available; may be
	// empty if the API did not expose transport-level details (e.g. vcsim).
	HBAInfo []HBAInfo
}

// HBAInfo describes a single host bus adapter relevant to storage transport.
type HBAInfo struct {
	Key   string // e.g. "vmhba0"
	Type  string // canonical type: FibreChannel, iSCSI, NVMe, VirtualSCSI, …
	Model string // optional human-readable model name
}

// classifyTransport returns the underlying transport protocol for a datastore
// based on its filesystem type and available HBA information. The returned set
// is { "FC", "iSCSI", "NVMe", "NFS", "unknown" }.
func classifyTransport(desc TransportDescriptor) string {
	if desc.FilesystemType == "" {
		return "unknown"
	}

	fs := strings.ToUpper(strings.TrimSpace(desc.FilesystemType))
	if fs == "NFS" || fs == "NFS40" || fs == "NFS41" || strings.HasPrefix(fs, "NFS") {
		return "NFS"
	}

	// For VMFS / VSAN / other filesystems the transport is determined from HBA
	// info. We scan all HBAs and pick the first meaningful match; in practice a
	// single datastore LUN is typically reachable over one transport, so any
	// non-VirtualSCSI HBA that exposes it is authoritative.
	hasFC := false
	hasiSCSI := false
	hasNVMe := false

	for _, hba := range desc.HBAInfo {
		t := strings.ToLower(strings.TrimSpace(hba.Type))
		switch t {
		case "fibrechannel", "fc":
			hasFC = true
		case "iscsi", "software-iscsi", "hardware-iscsi":
			hasiSCSI = true
		case "nvme", "nvmefc", "nvmftcp", "nvmetc", "non-volume", "nvme-of":
			hasNVMe = true
		}
	}

	if hasFC {
		return "FC"
	}
	if hasiSCSI {
		return "iSCSI"
	}
	if hasNVMe {
		return "NVMe"
	}
	return "unknown"
}

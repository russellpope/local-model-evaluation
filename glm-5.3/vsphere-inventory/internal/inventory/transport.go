package inventory

import "strings"

// Datastore transport protocols as reported in the datastores table.
const (
	TransportFC      = "FC"
	TransportISCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// LunDescriptor captures everything the transport classifier may look at for
// a single SCSI LUN (disk) that can back a VMFS datastore extent: the disk
// identity strings plus the types of the host bus adapters that provide
// paths to the LUN.
//
// The vSphere API reports adapter identities as key strings of the form
// "key-vim.host.FibreChannelHba-vmhba2"; AdapterTypes carries the decoded
// type component (e.g. "FibreChannelHba", "InternetScsiHba", "BlockHba").
type LunDescriptor struct {
	// CanonicalName is the SCSI canonical device name, e.g. "naa.600a0980...".
	CanonicalName string
	// DeviceName is the device path, e.g. "/vmfs/devices/disks/naa.600a0980...".
	DeviceName string
	// DeviceType is the SCSI device type, e.g. "disk".
	DeviceType string
	// Vendor and Model identify the physical array behind the LUN.
	Vendor string
	Model  string
	// AdapterTypes lists the HBA types whose paths reach this LUN.
	AdapterTypes []string
}

// ClassifyTransport maps a LUN/HBA descriptor to the underlying storage
// transport: FC, iSCSI or NVMe. It returns TransportUnknown when the
// descriptor carries no recognizable transport evidence (for example a
// local SAS/SATA/parallel-scsi disk, or missing adapter data).
//
// FC wins over iSCSI when a LUN is reachable through several fabric types,
// and fabric transports win over NVMe naming heuristics, because a fabric
// adapter is positive evidence while naming conventions are only a hint.
func ClassifyTransport(d LunDescriptor) string {
	has := func(list []string, needle string) bool {
		for _, s := range list {
			if strings.Contains(strings.ToLower(s), needle) {
				return true
			}
		}
		return false
	}

	switch {
	case has(d.AdapterTypes, "fibrechannelhba"):
		return TransportFC
	case has(d.AdapterTypes, "internetscsihba"):
		return TransportISCSI
	}

	if has(d.AdapterTypes, "nvme") {
		return TransportNVMe
	}
	// NVMe devices are commonly identified by their namespace naming
	// (e.g. "nvme.<controller>n<namespace>") or NVMe vendor/model strings.
	for _, s := range []string{d.CanonicalName, d.DeviceName, d.Vendor, d.Model} {
		if strings.Contains(strings.ToLower(s), "nvme") {
			return TransportNVMe
		}
	}

	return TransportUnknown
}

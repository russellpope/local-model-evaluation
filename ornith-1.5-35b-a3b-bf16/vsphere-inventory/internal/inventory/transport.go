package inventory

import "strings"

// transportDescriptor carries the signals used to decide a datastore's
// underlying storage transport: the filesystem type plus any backing device
// and host bus adapter hints.
type transportDescriptor struct {
	// FileSystem is the datastore filesystem type, e.g. "VMFS", "NFS", "OTHER".
	FileSystem string
	// Device is the backing device canonical name, e.g. "naa.6000..." or "nvme0n1".
	Device string
	// HBA is the host bus adapter type presenting the device: "fc", "iscsi", or "nvme".
	HBA string
}

// classifyTransport maps a transport descriptor to a protocol string: "FC",
// "iSCSI", "NVMe", "NFS", or "unknown". The filesystem type is authoritative
// for NFS; otherwise the host bus adapter and backing device decide between
// FC, iSCSI, and NVMe.
func classifyTransport(d transportDescriptor) string {
	fileSystem := strings.ToUpper(strings.TrimSpace(d.FileSystem))
	device := strings.ToLower(strings.TrimSpace(d.Device))
	hba := strings.ToLower(strings.TrimSpace(d.HBA))

	switch {
	case fileSystem == "NFS" || strings.Contains(device, "nfs"):
		return "NFS"
	case hba == "nvme" || strings.HasPrefix(device, "nvme"):
		return "NVMe"
	case hba == "iscsi":
		return "iSCSI"
	case hba == "fc" || hba == "scsi" ||
		strings.HasPrefix(device, "naa.") || strings.HasPrefix(device, "t10:") ||
		strings.HasPrefix(device, "eui."):
		return "FC"
	default:
		return "unknown"
	}
}

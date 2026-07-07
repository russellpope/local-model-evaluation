package transport

import (
	"strings"
)

// Classify determines the storage transport protocol from device/HBA descriptors.
// It inspects the host bus adapter type and device identifier strings to
// determine whether the backing storage is FC, iSCSI, NVMe, or unknown.
// For NFS datastores, the caller should pass an empty descriptor and the
// datastore type should already be known as NFS.
func Classify(hbaType string, deviceName string) string {
	combined := strings.ToLower(hbaType + " " + deviceName)

	if strings.Contains(combined, "nvme") {
		return "NVMe"
	}
	if strings.Contains(combined, "iscsi") {
		return "iSCSI"
	}
	if strings.Contains(combined, "fibre") || strings.Contains(combined, "fc ") ||
		strings.Contains(combined, "fcp") || strings.Contains(combined, "qla") ||
		strings.Contains(combined, "lpfc") || strings.Contains(combined, "emulex") {
		return "FC"
	}
	return "unknown"
}

// ClassifyDatastoreTransport determines the transport type for a datastore.
// If the datastore is NFS, it returns "NFS" directly. Otherwise it inspects
// the host bus adapter types and device names from all extents.
func ClassifyDatastoreTransport(isNFS bool, hbaTypes []string, deviceNames []string) string {
	if isNFS {
		return "NFS"
	}
	if len(hbaTypes) == 0 && len(deviceNames) == 0 {
		return "unknown"
	}

	// Classify each extent and return the first non-unknown result.
	for i := range hbaTypes {
		var deviceName string
		if i < len(deviceNames) {
			deviceName = deviceNames[i]
		}
		result := Classify(hbaTypes[i], deviceName)
		if result != "unknown" {
			return result
		}
	}
	// Fallback: try device names alone.
	for _, name := range deviceNames {
		result := Classify("", name)
		if result != "unknown" {
			return result
		}
	}
	return "unknown"
}

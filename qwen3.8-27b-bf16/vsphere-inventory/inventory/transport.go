package inventory

import "strings"

// Transport protocol labels reported by the datastores command.
const (
	TransportFC      = "FC"
	TransportISCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// HBADescriptor captures the observable attributes of the host bus adapter
// through which a datastore's backing storage device is attached:
//
//   - StorageProtocol is the HBA's storage protocol ("scsi", "nvme", ...);
//   - Kind is the HBA class (fibreChannel, internetScsi, block, ...);
//   - Model and Driver are the device model and driver strings.
type HBADescriptor struct {
	StorageProtocol string
	Kind            string
	Model           string
	Driver          string
}

// ClassifyTransport maps a datastore's filesystem type and the HBA descriptor
// of its backing storage device to a transport protocol:
//
//   - NFS datastores are always NFS, regardless of the HBA.
//   - An HBA reporting the nvme storage protocol means NVMe transport.
//   - A fibre channel HBA class (or a block HBA whose model/driver says
//     fibre channel) means FC.
//   - An internet SCSI HBA class (or a block HBA whose model/driver says
//     iSCSI or NVMe) means iSCSI / NVMe respectively.
//   - Anything else is unknown: the transport cannot be derived truthfully.
//
// The datastore's filesystem type (VMFS, NFS, local, ...) is deliberately NOT
// the answer: a single VMFS datastore may sit on FC, iSCSI or NVMe storage.
func ClassifyTransport(datastoreType string, hba HBADescriptor) string {
	if strings.EqualFold(strings.TrimSpace(datastoreType), "nfs") {
		return TransportNFS
	}

	if strings.EqualFold(strings.TrimSpace(hba.StorageProtocol), "nvme") {
		return TransportNVMe
	}

	text := strings.ToLower(hba.Model + " " + hba.Driver)
	kind := strings.ToLower(strings.TrimSpace(hba.Kind))

	switch kind {
	case "fibrechannel", "fibrechannelethernet", "fc", "fcoe":
		return TransportFC
	case "internetscsi", "iscsi":
		return TransportISCSI
	case "block":
		switch {
		case strings.Contains(text, "nvme"):
			return TransportNVMe
		case strings.Contains(text, "iscsi"):
			return TransportISCSI
		case strings.Contains(text, "fibre channel") || containsWord(text, "fc"):
			return TransportFC
		}
	}
	return TransportUnknown
}

// containsWord reports whether word appears as a standalone token of s.
func containsWord(s, word string) bool {
	for _, part := range strings.FieldsFunc(s, func(r rune) bool {
		return r != '-' && r != '.' && !isAlnum(r)
	}) {
		if part == word {
			return true
		}
	}
	return false
}

func isAlnum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

package transport

import "fmt"

type DeviceDescriptor struct {
	DeviceType string
	Model      string
	Vendor     string
}

func Classify(descriptor DeviceDescriptor) string {
	switch descriptor.DeviceType {
	case "NFS":
		return "NFS"
	case "FC":
		return "FC"
	case "iSCSI":
		return "iSCSI"
	case "NVMe":
		return "NVMe"
	case "VMFS", "VMDK":
		return "unknown"
	default:
		return "unknown"
	}
}

func ClassifyFromHBA(hbaType string) string {
	switch hbaType {
	case "FC":
		return "FC"
	case "iSCSI":
		return "iSCSI"
	case "NVMe":
		return "NVMe"
	case "NFS":
		return "NFS"
	default:
		return "unknown"
	}
}

func FormatError(transport string) error {
	return fmt.Errorf("unknown transport type: %s", transport)
}

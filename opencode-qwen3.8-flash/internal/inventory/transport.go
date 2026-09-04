package inventory

import "strings"

// Storage transport protocols reported by the datastores subcommand.
const (
	TransportFC      = "FC"
	TransportISCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// ClassifyTransport maps a device or host-bus-adapter descriptor to a storage
// transport protocol. It accepts either a concrete VIM type name (e.g.
// "HostInternetScsiHba", optionally as returned by "%T") or a storage device
// identifier (e.g. "iqn.1998-01.com.vmware:target", "eui.002538B4547D4889",
// "naa.6000c299..."). Anything not recognized degrades to TransportUnknown.
//
// HBA type names are the authoritative signal; device-name prefixes are a
// fallback for extents whose owning host cannot be inspected:
//   - Fibre Channel HBAs (incl. FCoE) and bare "naa." SCSI IDs -> FC
//   - software iSCSI initiators and "iqn."/"t10." names         -> iSCSI
//   - NVMe controllers and "eui."/"nqn." names                  -> NVMe
func ClassifyTransport(descriptor string) string {
	d := strings.ToLower(descriptor)
	d = strings.TrimPrefix(d, "*")
	d = strings.TrimPrefix(d, "types.")

	switch {
	case strings.Contains(d, "fibrechannel"):
		return TransportFC
	case strings.Contains(d, "iscsi"),
		strings.Contains(d, "internet-scsi"),
		strings.Contains(d, "internetscsi"),
		strings.HasPrefix(d, "iqn."),
		strings.HasPrefix(d, "t10."):
		return TransportISCSI
	case strings.Contains(d, "nvme"),
		strings.HasPrefix(d, "eui."),
		strings.HasPrefix(d, "nqn."):
		return TransportNVMe
	case strings.HasPrefix(d, "naa."):
		return TransportFC
	default:
		return TransportUnknown
	}
}

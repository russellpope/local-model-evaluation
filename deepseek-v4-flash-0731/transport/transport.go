// Package transport classifies a raw storage device or host-bus-adapter
// descriptor into an FC / iSCSI / NVMe transport, or "unknown".
//
// vCenter does not directly expose "this VMFS datastore runs over FC" —
// the transport has to be derived from the backing device(s) or host bus
// adapter(s), whose identity strings follow the conventions encoded here.
package transport

import "strings"

// Recognized transports (the datastore TYPE contract).
const (
	FC      = "FC"
	ISCSI   = "iSCSI"
	NVMe    = "NVMe"
	NFS     = "NFS"
	Unknown = "unknown"
)

// Classify maps a raw descriptor string to a transport protocol.
//
// Rules, checked in order:
//  1. NVMe: descriptor mentions "nvme" (e.g. "NvmeOverFc vmhba64", "t10.NVMe____").
//  2. FC: mentions "fibre", "fcoe", "fc " (delimited), or begins "naa."
//     (Fibre Channel WWNs).
//  3. iSCSI: mentions "iscsi", or begins "iqn." / "eui." (iSCSI target/LUN IDs).
//  4. Otherwise "unknown".
//
// Note the ordering is deliberate: an NVMe-over-Fibre device contains both
// "nvme" and "fc", and must classify as NVMe.
func Classify(descriptor string) string {
	d := strings.ToLower(strings.TrimSpace(descriptor))
	lower := func(s string) bool { return strings.Contains(d, s) }
	switch {
	case lower("nvme"):
		return NVMe
	case lower("fibre"), lower("fcoe"), strings.HasPrefix(d, "naa."), lower(" fc "):
		return FC
	case lower("iscsi"), strings.HasPrefix(d, "iqn."), strings.HasPrefix(d, "eui."):
		return ISCSI
	default:
		return Unknown
	}
}

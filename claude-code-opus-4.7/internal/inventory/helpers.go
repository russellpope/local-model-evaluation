package inventory

import (
	"fmt"
	"strings"
)

const (
	bytesPerGiB = int64(1) << 30
	bytesPerTiB = int64(1) << 40
)

// FormatBytes renders a byte count in GiB or TiB with one decimal place.
// Values of one TiB or more render as TiB; everything else renders as GiB so
// units stay consistent and greppable down a column.
func FormatBytes(b int64) string {
	if b >= bytesPerTiB {
		return fmt.Sprintf("%.1f TiB", float64(b)/float64(bytesPerTiB))
	}
	return fmt.Sprintf("%.1f GiB", float64(b)/float64(bytesPerGiB))
}

// ClassifyTransport maps a storage adapter/LUN descriptor to the underlying
// transport protocol. It inspects the concrete adapter type, driver, model,
// and any explicit protocol hint and returns FC, iSCSI, NVMe, or unknown.
// NFS is classified at the datastore level (by datastore type), not here,
// because NFS datastores are not backed by a SCSI HBA.
func ClassifyTransport(d StorageAdapterDescriptor) string {
	hay := strings.ToLower(strings.Join([]string{d.AdapterType, d.Driver, d.Model, d.Protocol}, " "))
	switch {
	case strings.Contains(hay, "nvme"):
		return TransportNVMe
	case containsAny(hay, "fibrechannel", "fibre channel", "fcoe"):
		return TransportFC
	case containsAny(hay, "iscsi", "internetscsi", "internet scsi"):
		return TransportISCSI
	default:
		return TransportUnknown
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

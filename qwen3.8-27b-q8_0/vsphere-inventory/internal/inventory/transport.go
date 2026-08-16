package inventory

import (
	"sort"
	"strings"
)

// Storage transport labels reported by the datastores command.
const (
	TransportFC      = "FC"
	TransportISCSI   = "iSCSI"
	TransportNVMe    = "NVMe"
	TransportNFS     = "NFS"
	TransportUnknown = "unknown"
)

// HbaInfo describes one host bus adapter observed on a host.
type HbaInfo struct {
	// Key is the HBA key, e.g. "vmhba1".
	Key string
	// Type is the concrete HBA class, e.g. "HostFibreChannelHba".
	Type string
	// StorageProtocol is the HBA's storage protocol field, e.g. "scsi"
	// or "nvme".
	StorageProtocol string
}

// HbaProtocol maps a single HBA descriptor to a storage transport. It
// returns TransportUnknown when the HBA does not identify an FC, iSCSI or
// NVMe fabric (for example SAS, SATA or USB adapters).
func HbaProtocol(h HbaInfo) string {
	switch h.Type {
	case "HostFibreChannelHba":
		return TransportFC
	case "HostInternetScsiHba":
		return TransportISCSI
	}
	if strings.EqualFold(h.StorageProtocol, "nvme") {
		return TransportNVMe
	}
	return TransportUnknown
}

// LunPath pairs a LUN device id (e.g. "naa.6001405...") with one storage
// path name observed on a host, e.g. "vmhba1:C0:T0:L0".
type LunPath struct {
	Device string
	Path   string
}

// ParsePathName splits a storage path name such as "vmhba1:C0:T0:L0" into
// the HBA key and the LUN address.
func ParsePathName(name string) (hbaKey, lun string, ok bool) {
	parts := strings.SplitN(name, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ClassifyTransport resolves the storage transport backing the given LUN
// devices using the host LUN paths and the HBA descriptors. It returns one
// of TransportFC, TransportISCSI, TransportNVMe or TransportUnknown.
//
// Each LUN path that leads to one of the requested devices casts a vote for
// the protocol of the HBA the path traverses. A single protocol wins; a
// dominant protocol wins over a minority; a tie is ambiguous and reported
// as TransportUnknown.
func ClassifyTransport(devices []string, luns []LunPath, hbas map[string]HbaInfo) string {
	want := make(map[string]bool, len(devices))
	for _, d := range devices {
		want[d] = true
	}

	votes := make(map[string]int)
	for _, lun := range luns {
		if !want[lun.Device] {
			continue
		}
		hbaKey, _, ok := ParsePathName(lun.Path)
		if !ok {
			continue
		}
		hba, ok := hbas[hbaKey]
		if !ok {
			continue
		}
		proto := HbaProtocol(hba)
		if proto == TransportUnknown {
			continue
		}
		votes[proto]++
	}

	if len(votes) == 0 {
		return TransportUnknown
	}
	if len(votes) == 1 {
		for proto := range votes {
			return proto
		}
	}

	// Multiple protocols: report the dominant one, unknown on a tie.
	protos := make([]string, 0, len(votes))
	for p := range votes {
		protos = append(protos, p)
	}
	sort.Strings(protos)

	best := protos[0]
	bestVotes := votes[best]
	tie := false
	for _, p := range protos[1:] {
		switch {
		case votes[p] > bestVotes:
			best, bestVotes, tie = p, votes[p], false
		case votes[p] == bestVotes:
			tie = true
		}
	}
	if tie {
		return TransportUnknown
	}
	return best
}

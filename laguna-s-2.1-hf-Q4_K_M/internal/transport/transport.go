package transport

import (
	"reflect"

	"github.com/vmware/govmomi/vim25/types"
)

type Transport string

const (
	TransportFC      Transport = "FC"
	TransportISCSI   Transport = "iSCSI"
	TransportNVMe    Transport = "NVMe"
	TransportNFS     Transport = "NFS"
	TransportUnknown Transport = "unknown"
)

func ClassifyHBA(hbaType string) Transport {
	switch hbaType {
	case "HostFibreChannelHba", "HostFibreChannelOverEthernetHba":
		return TransportFC
	case "HostInternetScsiHba", "HostTcpHba":
		return TransportISCSI
	case "HostPcieHba", "HostNvmeHba":
		return TransportNVMe
	default:
		return TransportUnknown
	}
}

func ClassifyHBAFromInterface(hba types.BaseHostHostBusAdapter) Transport {
	if hba == nil {
		return TransportUnknown
	}

	t := reflect.TypeOf(hba)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	return ClassifyHBA(t.Name())
}

func ClassifyNFSTransport() Transport {
	return TransportNFS
}

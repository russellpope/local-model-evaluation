package transport

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyHBA(t *testing.T) {
	tests := []struct {
		name     string
		hbaType  string
		expected Transport
	}{
		{"FC HBA", "HostFibreChannelHba", TransportFC},
		{"FCoE HBA", "HostFibreChannelOverEthernetHba", TransportFC},
		{"iSCSI HBA", "HostInternetScsiHba", TransportISCSI},
		{"TCP HBA", "HostTcpHba", TransportISCSI},
		{"PCIe HBA", "HostPcieHba", TransportNVMe},
		{"NVMe HBA", "HostNvmeHba", TransportNVMe},
		{"Block HBA", "HostBlockHba", TransportUnknown},
		{"Parallel SCSI HBA", "HostParallelScsiHba", TransportUnknown},
		{"RDMA HBA", "HostRdmaHba", TransportUnknown},
		{"SAS HBA", "HostSerialAttachedHba", TransportUnknown},
		{"empty", "", TransportUnknown},
		{"unknown type", "SomeUnknownType", TransportUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyHBA(tt.hbaType)
			if result != tt.expected {
				t.Errorf("ClassifyHBA(%q) = %q, want %q", tt.hbaType, result, tt.expected)
			}
		})
	}
}

func TestClassifyHBAFromInterface(t *testing.T) {
	tests := []struct {
		name     string
		hba      types.BaseHostHostBusAdapter
		expected Transport
	}{
		{"FC HBA", &types.HostFibreChannelHba{}, TransportFC},
		{"FCoE HBA", &types.HostFibreChannelOverEthernetHba{}, TransportFC},
		{"iSCSI HBA", &types.HostInternetScsiHba{}, TransportISCSI},
		{"TCP HBA", &types.HostTcpHba{}, TransportISCSI},
		{"PCIe HBA", &types.HostPcieHba{}, TransportNVMe},
		{"Block HBA", &types.HostBlockHba{}, TransportUnknown},
		{"Parallel SCSI HBA", &types.HostParallelScsiHba{}, TransportUnknown},
		{"nil", nil, TransportUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyHBAFromInterface(tt.hba)
			if result != tt.expected {
				t.Errorf("ClassifyHBAFromInterface() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestClassifyNFSTransport(t *testing.T) {
	result := ClassifyNFSTransport()
	if result != TransportNFS {
		t.Errorf("ClassifyNFSTransport() = %q, want %q", result, TransportNFS)
	}
}

func TestTransportConstants(t *testing.T) {
	tests := []struct {
		name      string
		transport Transport
		expected  string
	}{
		{"FC", TransportFC, "FC"},
		{"iSCSI", TransportISCSI, "iSCSI"},
		{"NVMe", TransportNVMe, "NVMe"},
		{"NFS", TransportNFS, "NFS"},
		{"unknown", TransportUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.transport) != tt.expected {
				t.Errorf("Transport %q = %q, want %q", tt.name, tt.transport, tt.expected)
			}
		})
	}
}

func TestClassifyHBAFromInterfaceUnknownTypes(t *testing.T) {
	tests := []struct {
		name     string
		hba      types.BaseHostHostBusAdapter
		expected Transport
	}{
		{"BlockHba", &types.HostBlockHba{}, TransportUnknown},
		{"ParallelScsiHba", &types.HostParallelScsiHba{}, TransportUnknown},
		{"RdmaHba", &types.HostRdmaHba{}, TransportUnknown},
		{"SerialAttachedHba", &types.HostSerialAttachedHba{}, TransportUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyHBAFromInterface(tt.hba)
			if result != tt.expected {
				t.Errorf("ClassifyHBAFromInterface() = %q, want %q", result, tt.expected)
			}
		})
	}
}

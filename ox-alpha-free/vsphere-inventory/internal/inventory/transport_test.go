package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name  string
		kinds []string
		want  string
	}{
		{"FC only", []string{KindFC}, "FC"},
		{"iSCSI only", []string{KindiSCSI}, "iSCSI"},
		{"NVMe only", []string{KindNVMe}, "NVMe"},
		{"multi-path FC wins", []string{KindiSCSI, KindFC}, "FC"},
		{"multi-path iSCSI beats NVMe", []string{KindNVMe, KindiSCSI}, "iSCSI"},
		{"local devices only", []string{""}, "unknown"},
		{"no paths", nil, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyTransport(tt.kinds); got != tt.want {
				t.Errorf("ClassifyTransport(%v) = %q, want %q", tt.kinds, got, tt.want)
			}
		})
	}
}

// Representative device descriptors: an FC HBA, a software iSCSI HBA, and the
// NVMe-over-TCP adapter type vSphere uses for NVMe/TCP.
func representativeHBAs() map[string]types.BaseHostHostBusAdapter {
	return map[string]types.BaseHostHostBusAdapter{
		"fc": &types.HostFibreChannelHba{
			HostHostBusAdapter: types.HostHostBusAdapter{Device: "vmhba1", Model: "Emulex"},
		},
		"fcoe": &types.HostFibreChannelOverEthernetHba{
			HostFibreChannelHba: types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{Device: "vmhba2"},
			},
		},
		"iscsi": &types.HostInternetScsiHba{
			HostHostBusAdapter: types.HostHostBusAdapter{Device: "vmhba65"},
		},
		"nvme-tcp": &types.HostTcpHba{
			HostHostBusAdapter: types.HostHostBusAdapter{Device: "vmhba66"},
		},
		"nvme-pcie": &types.HostPcieHba{
			HostHostBusAdapter: types.HostHostBusAdapter{Device: "vmhba3"},
		},
		"local-sas": &types.HostSerialAttachedHba{
			HostHostBusAdapter: types.HostHostBusAdapter{Device: "vmhba4"},
		},
	}
}

func TestHBAKind(t *testing.T) {
	hbas := representativeHBAs()
	tests := []struct {
		name string
		key  string
		want string
	}{
		{"fibre channel", "fc", KindFC},
		{"fcoe counts as fc", "fcoe", KindFC},
		{"iscsi", "iscsi", KindiSCSI},
		{"nvme over tcp", "nvme-tcp", KindNVMe},
		{"nvme pcie", "nvme-pcie", KindNVMe},
		{"local SAS is not a fabric", "local-sas", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hbaKind(hbas[tt.key]); got != tt.want {
				t.Errorf("hbaKind(%s) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestTargetTransportKind(t *testing.T) {
	tests := []struct {
		name string
		in   types.BaseHostTargetTransport
		want string
	}{
		{"fc target", &types.HostFibreChannelTargetTransport{}, KindFC},
		{"iscsi target", &types.HostInternetScsiTargetTransport{}, KindiSCSI},
		{"nvme tcp target", &types.HostTcpTargetTransport{}, KindNVMe},
		{"parallel scsi target", &types.HostParallelScsiTargetTransport{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := targetTransportKind(tt.in); got != tt.want {
				t.Errorf("targetTransportKind(%T) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

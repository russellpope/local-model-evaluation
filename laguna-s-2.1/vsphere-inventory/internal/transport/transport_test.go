package transport

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		desc     DeviceDescriptor
		expected string
	}{
		{"NFS", DeviceDescriptor{DeviceType: "NFS"}, "NFS"},
		{"NFS41", DeviceDescriptor{DeviceType: "NFS41"}, "NFS"},
		{"FC", DeviceDescriptor{DeviceType: "FC"}, "unknown"},
		{"iSCSI", DeviceDescriptor{DeviceType: "iSCSI"}, "unknown"},
		{"NVMe", DeviceDescriptor{DeviceType: "NVMe"}, "unknown"},
		{"VMFS", DeviceDescriptor{DeviceType: "VMFS"}, "unknown"},
		{"VMDK", DeviceDescriptor{DeviceType: "VMDK"}, "unknown"},
		{"unknown type", DeviceDescriptor{DeviceType: "unknown"}, "unknown"},
		{"empty", DeviceDescriptor{DeviceType: ""}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Classify(tt.desc)
			if result != tt.expected {
				t.Errorf("Classify(%+v) = %q, want %q", tt.desc, result, tt.expected)
			}
		})
	}
}

func TestClassifyHBA(t *testing.T) {
	tests := []struct {
		name string
		hba  types.BaseHostHostBusAdapter
		want string
	}{
		{
			name: "FibreChannelHba",
			hba: &types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.FibreChannelHba-vmhba1",
				},
			},
			want: "FC",
		},
		{
			name: "InternetScsiHba",
			hba: &types.HostInternetScsiHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.InternetScsiHba-vmhba2",
				},
			},
			want: "iSCSI",
		},
		{
			name: "ParallelScsiHba (unknown)",
			hba: &types.HostParallelScsiHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.ParallelScsiHba-vmhba0",
				},
			},
			want: "unknown",
		},
		{
			name: "BlockHba (unknown)",
			hba: &types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key: "key-vim.host.BlockHba-vmhba3",
				},
			},
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyHBA(tt.hba)
			if got != tt.want {
				t.Errorf("classifyHBA() = %q, want %q", got, tt.want)
			}
		})
	}
}

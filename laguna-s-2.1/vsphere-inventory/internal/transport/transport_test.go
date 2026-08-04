package transport

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

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
		{
			name: "NvmeViaStorageProtocol",
			hba: &types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:             "key-vim.host.BlockHba-vmhba4",
					StorageProtocol: "nvme",
				},
			},
			want: "NVMe",
		},
		{
			name: "FcoeViaStorageProtocol",
			hba: &types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{
					Key:             "key-vim.host.BlockHba-vmhba5",
					StorageProtocol: "fcoe",
				},
			},
			want: "FCoE",
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

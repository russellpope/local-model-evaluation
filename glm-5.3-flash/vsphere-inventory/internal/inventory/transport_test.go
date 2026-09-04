package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestHBADescriptor(t *testing.T) {
	tests := []struct {
		name string
		hba  types.BaseHostHostBusAdapter
		want string
	}{
		{"fibre channel", &types.HostFibreChannelHba{}, "fibreChannel"},
		{"fibre channel over ethernet", &types.HostFibreChannelOverEthernetHba{}, "fcoe"},
		{"software iSCSI", &types.HostInternetScsiHba{}, "internetScsi"},
		{"PCIe NVMe", &types.HostPcieHba{}, "pcie"},
		{"NVMe over TCP", &types.HostTcpHba{}, "tcp"},
		{"local parallel SCSI", &types.HostParallelScsiHba{}, "parallelScsi"},
		{"serial attached SCSI", &types.HostSerialAttachedHba{}, "serialAttached"},
		{"RDMA", &types.HostRdmaHba{}, "rdma"},
		{"IDE/block", &types.HostBlockHba{}, "block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HBADescriptor(tt.hba); got != tt.want {
				t.Errorf("HBADescriptor(%T) = %q, want %q", tt.hba, got, tt.want)
			}
		})
	}
}

func TestTransportFromHBADescriptor(t *testing.T) {
	tests := []struct {
		desc string
		want string
	}{
		{"fibreChannel", "FC"},
		{"fcoe", "FC"},
		{"internetScsi", "iSCSI"},
		{"pcie", "NVMe"},
		{"tcp", "NVMe"},
		{"parallelScsi", "unknown"},
		{"serialAttached", "unknown"},
		{"rdma", "unknown"},
		{"block", "unknown"},
		{"", "unknown"},
		{"made-up", "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if got := TransportFromHBADescriptor(tt.desc); got != tt.want {
				t.Errorf("TransportFromHBADescriptor(%q) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

func TestTransportForExtents(t *testing.T) {
	lunByCanonical := map[string]string{
		"naa.600508e0000000001": "key-vim.host.ScsiDisk-fc1",
		"naa.6000c29local1":     "key-vim.host.ScsiDisk-local1",
	}
	adapterByLun := map[string]string{
		"key-vim.host.ScsiDisk-fc1":    "key-vim.host.FibreChannelHba-vmhba1",
		"key-vim.host.ScsiDisk-local1": "key-vim.host.ParallelScsiHba-vmhba0",
	}
	descByAdapter := map[string]string{
		"key-vim.host.FibreChannelHba-vmhba1":  "fibreChannel",
		"key-vim.host.ParallelScsiHba-vmhba0":  "parallelScsi",
		"key-vim.host.InternetScsiHba-vmhba65": "internetScsi",
	}

	tests := []struct {
		name    string
		extents []types.HostScsiDiskPartition
		want    string
	}{
		{
			name:    "FC-backed extent",
			extents: []types.HostScsiDiskPartition{{DiskName: "naa.600508e0000000001", Partition: 1}},
			want:    "FC",
		},
		{
			name: "first known transport wins over unknown one",
			extents: []types.HostScsiDiskPartition{
				{DiskName: "naa.6000c29local1", Partition: 3},
				{DiskName: "naa.600508e0000000001", Partition: 1},
			},
			want: "FC",
		},
		{
			name:    "unresolvable disk name yields no transport",
			extents: []types.HostScsiDiskPartition{{DiskName: "____simulated_volumes_____"}},
			want:    "",
		},
		{
			name:    "no extents yields no transport",
			extents: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TransportForExtents(tt.extents, lunByCanonical, adapterByLun, descByAdapter)
			if got != tt.want {
				t.Errorf("TransportForExtents() = %q, want %q", got, tt.want)
			}
		})
	}
}

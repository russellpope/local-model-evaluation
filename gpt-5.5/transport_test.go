package main

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyTransportDescriptor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want StorageTransport
	}{
		{name: "fibre channel", in: "HostFibreChannelHba vmhba2 fc.20000025b5000000", want: TransportFC},
		{name: "iscsi", in: "HostInternetScsiHba iqn.1998-01.com.vmware:host", want: TransportISCSI},
		{name: "nvme", in: "HostNvmeHba vmhba64 nvme controller", want: TransportNVMe},
		{name: "nfs", in: "NFS remote datastore", want: TransportNFS},
		{name: "unknown", in: "local sata disk", want: TransportUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyTransportDescriptor(tt.in); got != tt.want {
				t.Fatalf("ClassifyTransportDescriptor(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestVMFSTransportFromHostStorage(t *testing.T) {
	tests := []struct {
		name    string
		disk    string
		storage types.HostStorageDeviceInfo
		want    StorageTransport
	}{
		{
			name: "fibre channel",
			disk: "naa.fc",
			storage: vmfsStorage("naa.fc", "lun-fc", &types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{Key: "hba-fc", Device: "vmhba2"},
			}),
			want: TransportFC,
		},
		{
			name: "iscsi",
			disk: "naa.iscsi",
			storage: vmfsStorage("naa.iscsi", "lun-iscsi", &types.HostInternetScsiHba{
				HostHostBusAdapter: types.HostHostBusAdapter{Key: "hba-iscsi", Device: "vmhba64"},
				IScsiName:          "iqn.1998-01.com.vmware:host",
			}),
			want: TransportISCSI,
		},
		{
			name: "nvme storage protocol",
			disk: "eui.nvme",
			storage: vmfsStorage("eui.nvme", "lun-nvme", &types.HostHostBusAdapter{
				Key:             "hba-nvme",
				Device:          "vmhba65",
				StorageProtocol: "NVMe",
			}),
			want: TransportNVMe,
		},
		{
			name: "unknown block hba",
			disk: "naa.sas",
			storage: vmfsStorage("naa.sas", "lun-sas", &types.HostBlockHba{
				HostHostBusAdapter: types.HostHostBusAdapter{Key: "hba-sas", Device: "vmhba0", StorageProtocol: "SAS"},
			}),
			want: TransportUnknown,
		},
		{
			name: "missing lun",
			disk: "naa.missing",
			storage: vmfsStorage("naa.other", "lun-other", &types.HostFibreChannelHba{
				HostHostBusAdapter: types.HostHostBusAdapter{Key: "hba-fc", Device: "vmhba2"},
			}),
			want: TransportUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &types.VmfsDatastoreInfo{
				Vmfs: &types.HostVmfsVolume{
					Extent: []types.HostScsiDiskPartition{{DiskName: tt.disk}},
				},
			}
			if got := vmfsTransport(info, []types.HostStorageDeviceInfo{tt.storage}); got != tt.want {
				t.Fatalf("vmfsTransport(%q) = %q, want %q", tt.disk, got, tt.want)
			}
		})
	}
}

func vmfsStorage(diskName, lunKey string, hba types.BaseHostHostBusAdapter) types.HostStorageDeviceInfo {
	hbaKey := hba.GetHostHostBusAdapter().Key
	return types.HostStorageDeviceInfo{
		HostBusAdapter: []types.BaseHostHostBusAdapter{hba},
		ScsiLun: []types.BaseScsiLun{
			&types.HostScsiDisk{
				ScsiLun: types.ScsiLun{
					Key:           lunKey,
					CanonicalName: diskName,
				},
			},
		},
		ScsiTopology: &types.HostScsiTopology{
			Adapter: []types.HostScsiTopologyInterface{
				{
					Adapter: hbaKey,
					Target: []types.HostScsiTopologyTarget{
						{
							Lun: []types.HostScsiTopologyLun{
								{ScsiLun: lunKey},
							},
						},
					},
				},
			},
		},
	}
}

package inventory

import (
	"testing"
)

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name string
		in   LunDescriptor
		want string
	}{
		{
			name: "fibre channel adapter",
			in: LunDescriptor{
				CanonicalName: "naa.600508b1001c6e1e2b3d4f5a6b7c8d9e",
				DeviceName:    "/vmfs/devices/disks/naa.600508b1001c6e1e2b3d4f5a6b7c8d9e",
				DeviceType:    "disk",
				Vendor:        "HP",
				Model:         "LOGICAL VOLUME",
				AdapterTypes:  []string{"FibreChannelHba"},
			},
			want: TransportFC,
		},
		{
			name: "fibre channel key-style adapter string",
			in: LunDescriptor{
				CanonicalName: "naa.600a0980383034464b4c5d6e7f8090a1",
				AdapterTypes:  []string{"key-vim.host.FibreChannelHba-vmhba1"},
			},
			want: TransportFC,
		},
		{
			name: "iscsi adapter",
			in: LunDescriptor{
				CanonicalName: "naa.6001405f4c8e2d7a6b5c4d3e2f1a0b9c",
				DeviceName:    "/vmfs/devices/disks/naa.6001405f4c8e2d7a6b5c4d3e2f1a0b9c",
				Vendor:        "NETAPP",
				Model:         "LUN",
				AdapterTypes:  []string{"InternetScsiHba"},
			},
			want: TransportISCSI,
		},
		{
			name: "iscsi key-style adapter string",
			in: LunDescriptor{
				CanonicalName: "naa.624a9370c8f0a3b2c1d0e9f8a7b6c5d4",
				AdapterTypes:  []string{"key-vim.host.InternetScsiHba-vmhba33"},
			},
			want: TransportISCSI,
		},
		{
			name: "nvme device naming",
			in: LunDescriptor{
				CanonicalName: "nvme.1234-5678-90ab-cdef-1122-3344-5566-7788",
				DeviceName:    "/vmfs/devices/disks/nvme.1234-5678-90ab-cdef",
				DeviceType:    "disk",
				Vendor:        "SAMSUNG",
				Model:         "MZPLL1T9HAJQ0D3",
				AdapterTypes:  []string{"BlockHba"},
			},
			want: TransportNVMe,
		},
		{
			name: "nvme vendor model",
			in: LunDescriptor{
				CanonicalName: "eui.0025388b710114e1",
				DeviceType:    "disk",
				Vendor:        "NVME",
				Model:         "SSD Controller",
				AdapterTypes:  []string{"BlockHba"},
			},
			want: TransportNVMe,
		},
		{
			name: "nvme adapter type",
			in: LunDescriptor{
				CanonicalName: "eui.0025388b710114e1",
				AdapterTypes:  []string{"key-vim.host.NVMeController-vmhba2"},
			},
			want: TransportNVMe,
		},
		{
			name: "fc wins over iscsi",
			in: LunDescriptor{
				CanonicalName: "naa.624a9370c8f0a3b2c1d0e9f8a7b6c5d4",
				AdapterTypes:  []string{"InternetScsiHba", "FibreChannelHba"},
			},
			want: TransportFC,
		},
		{
			name: "fabric wins over nvme naming",
			in: LunDescriptor{
				CanonicalName: "naa.624a9370nvmelookalike",
				AdapterTypes:  []string{"InternetScsiHba"},
			},
			want: TransportISCSI,
		},
		{
			name: "local parallel scsi disk is unknown",
			in: LunDescriptor{
				CanonicalName: "mpx.vmhba0:C0:T0:L0",
				DeviceName:    "/vmfs/devices/disks/mpx.vmhba0:C0:T0:L0",
				DeviceType:    "disk",
				Vendor:        "VMware",
				Model:         "Virtual disk",
				AdapterTypes:  []string{"ParallelScsiHba"},
			},
			want: TransportUnknown,
		},
		{
			name: "no evidence at all",
			in:   LunDescriptor{},
			want: TransportUnknown,
		},
		{
			name: "local block hba only",
			in: LunDescriptor{
				CanonicalName: "t10.ATA_____VMware_Virtual_S",
				AdapterTypes:  []string{"BlockHba"},
			},
			want: TransportUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyTransport(tt.in); got != tt.want {
				t.Errorf("ClassifyTransport(%+v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

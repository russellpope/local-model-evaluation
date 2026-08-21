package inventory

import "testing"

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name string
		desc transportDescriptor
		want string
	}{
		{"fc_hba_naadevice", transportDescriptor{FileSystem: "VMFS", HBA: "fc", Device: "naa.6000c50012345678"}, "FC"},
		{"scsi_hba_naadevice", transportDescriptor{FileSystem: "VMFS", HBA: "scsi", Device: "naa.5000c50012345678"}, "FC"},
		{"fc_case_insensitive", transportDescriptor{FileSystem: "vmfs", HBA: "FC", Device: "NAA.6000c50012345678"}, "FC"},
		{"t10_device_is_fc", transportDescriptor{FileSystem: "VMFS", HBA: "fc", Device: "t10.NAWSVGJ783456"}, "FC"},
		{"iscsi_hba", transportDescriptor{FileSystem: "VMFS", HBA: "iscsi", Device: "naa.6000c50012345678"}, "iSCSI"},
		{"nvme_hba", transportDescriptor{FileSystem: "VMFS", HBA: "nvme", Device: "nvme0n1"}, "NVMe"},
		{"nvme_device_prefix", transportDescriptor{FileSystem: "VMFS", HBA: "", Device: "nvme.NVME0VMWARE123"}, "NVMe"},
		{"nfs_filesystem", transportDescriptor{FileSystem: "NFS", Device: "10.0.0.1:/datastore"}, "NFS"},
		{"nfs_from_device", transportDescriptor{FileSystem: "VMFS", Device: "nfs://host/data"}, "NFS"},
		{"vmfs_no_hints", transportDescriptor{FileSystem: "VMFS"}, "unknown"},
		{"empty", transportDescriptor{}, "unknown"},
		{"other_filesystem_no_hints", transportDescriptor{FileSystem: "OTHER"}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTransport(tt.desc); got != tt.want {
				t.Errorf("classifyTransport(%+v) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

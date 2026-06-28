package inventory

import "testing"

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name string
		desc TransportDescriptor
		want string
	}{
		{
			name: "nfs_filesystem",
			desc: TransportDescriptor{FilesystemType: "NFS"},
			want: "NFS",
		},
		{
			name: "nfs41_filesystem",
			desc: TransportDescriptor{FilesystemType: "NFS41"},
			want: "NFS",
		},
		{
			name: "vmfs_with_fc_hba",
			desc: TransportDescriptor{
				FilesystemType: "VMFS",
				HBAInfo:        []HBAInfo{{Key: "vmhba0", Type: "FibreChannel"}},
			},
			want: "FC",
		},
		{
			name: "vmfs_with_iscsi_hba",
			desc: TransportDescriptor{
				FilesystemType: "VMFS",
				HBAInfo:        []HBAInfo{{Key: "vmhba1", Type: "iSCSI"}},
			},
			want: "iSCSI",
		},
		{
			name: "vmfs_with_nvme_hba",
			desc: TransportDescriptor{
				FilesystemType: "VMFS",
				HBAInfo:        []HBAInfo{{Key: "vmhba2", Type: "NVMe"}},
			},
			want: "NVMe",
		},
		{
			name: "vmfs_with_multiple_hbas_picks_fc",
			desc: TransportDescriptor{
				FilesystemType: "VMFS",
				HBAInfo: []HBAInfo{
					{Key: "vmhba0", Type: "FibreChannel"},
					{Key: "vmhba1", Type: "iSCSI"},
				},
			},
			want: "FC", // FC takes priority in our classifier
		},
		{
			name: "empty_filesystem_no_hbas",
			desc: TransportDescriptor{},
			want: "unknown",
		},
		{
			name: "vmfs_no_hba_info",
			desc: TransportDescriptor{FilesystemType: "VMFS"},
			want: "unknown",
		},
		{
			name: "nfs_with_extra_hbas_still_nfs",
			desc: TransportDescriptor{
				FilesystemType: "NFS",
				HBAInfo:        []HBAInfo{{Key: "vmhba0", Type: "FibreChannel"}},
			},
			want: "NFS", // NFS short-circuits before HBA inspection
		},
		{
			name: "nvmetcp_hba",
			desc: TransportDescriptor{
				FilesystemType: "VMFS",
				HBAInfo:        []HBAInfo{{Key: "vmhba3", Type: "NVMETC"}},
			},
			want: "NVMe",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := classifyTransport(tt.desc)
			if got != tt.want {
				t.Errorf("classifyTransport(%+v) = %q, want %q", tt.desc, got, tt.want)
			}
		})
	}
}

func TestClassifyTransport_UnknownType(t *testing.T) {
	desc := TransportDescriptor{FilesystemType: "unknown-fs"}
	got := classifyTransport(desc)
	if got != "unknown" {
		t.Errorf("classifyTransport with unknown filesystem = %q, want %q", got, "unknown")
	}
}

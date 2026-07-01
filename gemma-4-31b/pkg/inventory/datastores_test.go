package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name     string
		desc     TransportDescriptor
		expected string
	}{
		{
			name: "NFS (Info)",
			desc: TransportDescriptor{
				Info: &types.NasDatastoreInfo{},
			},
			expected: "NFS",
		},
		{
			name: "NFS (Summary)",
			desc: TransportDescriptor{
				SummaryType: "NFS",
			},
			expected: "NFS",
		},
		{
			name: "NFS41 (Summary)",
			desc: TransportDescriptor{
				SummaryType: "NFS41",
			},
			expected: "NFS",
		},
		{
			name: "FC (Adapter)",
			desc: TransportDescriptor{
				Info:        &types.VmfsDatastoreInfo{},
				AdapterInfo: "vmw_fc",
			},
			expected: "FC",
		},
		{
			name: "iSCSI (Adapter)",
			desc: TransportDescriptor{
				Info:        &types.VmfsDatastoreInfo{},
				AdapterInfo: "vmw_iscsi",
			},
			expected: "iSCSI",
		},
		{
			name: "NVMe (Adapter)",
			desc: TransportDescriptor{
				Info:        &types.VmfsDatastoreInfo{},
				AdapterInfo: "vmw_nvme",
			},
			expected: "NVMe",
		},
		{
			name: "VMFS Unknown (No Adapter)",
			desc: TransportDescriptor{
				Info: &types.VmfsDatastoreInfo{},
			},
			expected: "unknown",
		},
		{
			name: "Unknown",
			desc: TransportDescriptor{
				Info: nil,
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTransport(tt.desc); got != tt.expected {
				t.Errorf("classifyTransport() = %v, want %v", got, tt.expected)
			}
		})
	}
}

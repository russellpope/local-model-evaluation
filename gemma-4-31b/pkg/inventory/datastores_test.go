package inventory

import (
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name     string
		info     interface{}
		expected string
	}{
		{
			name:     "NFS",
			info:     &types.NfsDatastoreInfo{},
			expected: "NFS",
		},
		{
			name:     "VMFS",
			info:     &types.VmfsDatastoreInfo{},
			expected: "VMFS",
		},
		{
			name:     "Unknown",
			info:     nil,
			expected: "unknown",
		},
		{
			name:     "Other",
			info:     &types.VirtualDisk{},
			expected: "VMFS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := mo.Datastore{
				Info: tt.info,
			}
			if got := classifyTransport(ds); got != tt.expected {
				t.Errorf("classifyTransport() = %v, want %v", got, tt.expected)
			}
		})
	}
}

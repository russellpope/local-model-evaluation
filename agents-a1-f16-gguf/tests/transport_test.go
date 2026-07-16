package tests

import (
	"testing"

	"github.com/vmware/govmomi/vim25/types"
	"vsphere-inventory/internal/storage"
)

// TestTransportClassifier tests the pure function that classifies storage transport.
// Since vcsim doesn't provide rich HBA information, we test the logic directly.
func TestTransportClassifier(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		backingType string
		url         string
		expected    string
	}{
		{
			name:        "NFS backing",
			backingType: "nfs",
			url:         "nfs://server/share",
			expected:    "NFS",
		},
		{
			name:        "VMFS with iSCSI URL",
			backingType: "vmfs",
			url:         "ds-123",
			expected:    "unknown", // vcsim doesn't provide HBA info
		},
		{
			name:        "VMFS with NFS URL",
			backingType: "vmfs",
			url:         "nfs://server/share",
			expected:    "unknown",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a minimal DatastoreInfo for testing
			ds := &types.DatastoreInfo{
				Name: tc.name,
				Url:  tc.url,
			}

			// Set the appropriate backing based on backingType
			switch tc.backingType {
			case "nfs":
				ds.Nfs = &types.NFSDatastoreInfo{}
			case "vmfs":
				ds.Vmfs = &types.VMFSDatastoreInfo{}
			}

			transport := storage.ClassifyTransport(ds)
			if transport != tc.expected {
				t.Errorf("ClassifyTransport(%s) = %s; want %s", tc.name, transport, tc.expected)
			}
		})
	}
}

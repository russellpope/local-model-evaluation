package inventory

import (
	"testing"

	"github.com/example/vsphere-inventory/internal/formatter"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0.0 GiB"},
		{1024 * 1024 * 1024 - 1, "1.0 GiB"}, // rounds up or down? prompt says one decimal place.
		{1024 * 1024 * 1024, "1.0 GiB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TiB"},
		{1024 * 1024 * 1024 * 1024 * 2, "2.0 TiB"},
	}

	for _, tt := range tests {
		got := formatter.FormatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("FormatBytes(%d) = %s; want %s", tt.input, got, tt.expected)
		}
	}
}

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		deviceType  string
		adapterType string
		expected    string
	}{
		{"nfs_device", "generic", "NFS"},
		{"generic", "fc_hba", "FC"},
		{"iscsi_ln", "generic", "iSCSI"},
		{"nvme_drive", "generic", "NVMe"},
		{"generic", "generic", "unknown"},
	}

	for _, tt := range tests {
		got := ClassifyTransport(tt.deviceType, tt.adapterType)
		if got != tt.expected {
			t.Errorf("ClassifyTransport(%s, %s) = %s; want %s", tt.deviceType, tt.adapterType, got, tt.expected)
		}
	}
}

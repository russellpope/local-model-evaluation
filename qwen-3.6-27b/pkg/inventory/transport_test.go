package inventory

import "testing"

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"fc", "FC"},
		{"FC", "FC"},
		{"fcqe", "FC"},
		{"iscsi", "iSCSI"},
		{"iSCSI", "iSCSI"},
		{"softwareiscsi", "iSCSI"},
		{"hardwareiscsi", "iSCSI"},
		{"nvme", "NVMe"},
		{"NVMe", "NVMe"},
		{"sata", "SATA"},
		{"usb", "USB"},
		{"unknown", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ClassifyTransport(tt.input)
			if result != tt.expected {
				t.Errorf("ClassifyTransport(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

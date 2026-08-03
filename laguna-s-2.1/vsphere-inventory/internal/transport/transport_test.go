package transport

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		desc     DeviceDescriptor
		expected string
	}{
		{"NFS", DeviceDescriptor{DeviceType: "NFS"}, "NFS"},
		{"FC", DeviceDescriptor{DeviceType: "FC"}, "FC"},
		{"iSCSI", DeviceDescriptor{DeviceType: "iSCSI"}, "iSCSI"},
		{"NVMe", DeviceDescriptor{DeviceType: "NVMe"}, "NVMe"},
		{"VMFS", DeviceDescriptor{DeviceType: "VMFS"}, "unknown"},
		{"VMDK", DeviceDescriptor{DeviceType: "VMDK"}, "unknown"},
		{"unknown type", DeviceDescriptor{DeviceType: "unknown"}, "unknown"},
		{"empty", DeviceDescriptor{DeviceType: ""}, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Classify(tt.desc)
			if result != tt.expected {
				t.Errorf("Classify(%+v) = %q, want %q", tt.desc, result, tt.expected)
			}
		})
	}
}

func TestClassifyFromHBA(t *testing.T) {
	tests := []struct {
		name     string
		hbaType  string
		expected string
	}{
		{"FC HBA", "FC", "FC"},
		{"iSCSI HBA", "iSCSI", "iSCSI"},
		{"NVMe HBA", "NVMe", "NVMe"},
		{"NFS HBA", "NFS", "NFS"},
		{"unknown HBA", "unknown", "unknown"},
		{"empty HBA", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyFromHBA(tt.hbaType)
			if result != tt.expected {
				t.Errorf("ClassifyFromHBA(%q) = %q, want %q", tt.hbaType, result, tt.expected)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	err := FormatError("unknown")
	if err == nil {
		t.Error("FormatError should return a non-nil error")
	}
	if err.Error() != "unknown transport type: unknown" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
}

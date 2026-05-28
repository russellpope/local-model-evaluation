package transport_test

import (
	"testing"

	"vsphere-inventory/internal/transport"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		device   string
		expected string
	}{
		{"empty", "", "unknown"},
		{"FC NAA simple", "NAA:500143800b123456", "FC"},
		{"iSCSI NAA with IP", "NAA:500143800b123456:IP:192.168.1.1", "iSCSI"},
		{"FC T10 with WWN", "T10:WWN.500143800b123456", "FC"},
		{"VMHBA FC", "vmhba0:C0:T0:L0", "FC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transport.Classify(tt.device)
			if got != tt.expected {
				t.Errorf("Classify(%q) = %q, want %q", tt.device, got, tt.expected)
			}
		})
	}
}

func TestClassifyCaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		device   string
		expected string
	}{
		{"lowercase NAA", "naa:500143800b123456", "FC"},
		{"lowercase VMHBA", "vmhba0:c0:t0:l0", "FC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transport.Classify(tt.device)
			if got != tt.expected {
				t.Errorf("Classify(%q) = %q, want %q", tt.device, got, tt.expected)
			}
		})
	}
}

func TestClassifyUnknown(t *testing.T) {
	tests := []struct {
		name   string
		device string
	}{
		{"random string", "random-string-12345"},
		{"unknown prefix", "XYZ:500143800b123456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transport.Classify(tt.device)
			if got != "unknown" {
				t.Errorf("Classify(%q) = %q, want unknown", tt.device, got)
			}
		})
	}
}

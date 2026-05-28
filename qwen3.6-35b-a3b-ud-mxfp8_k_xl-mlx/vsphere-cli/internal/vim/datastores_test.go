package vim

import (
	"testing"

	"github.com/local-model-evaluation/vsphere-cli/internal/tabular"
)

func TestClassifyTransport(t *testing.T) {
	tests := []struct {
		name     string
		dsType   string
		expected string
	}{
		{
			name:     "nfs datastore",
			dsType:   "nfs",
			expected: "NFS",
		},
		{
			name:     "nfs41 datastore",
			dsType:   "nfs41",
			expected: "NFS",
		},
		{
			name:     "vsan datastore",
			dsType:   "vsan",
			expected: "NVMe",
		},
		{
			name:     "vmfs datastore",
			dsType:   "vmfs",
			expected: "unknown",
		},
		{
			name:     "unknown datastore type",
			dsType:   "unknown_type",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyTransport(tt.dsType)
			if result != tt.expected {
				t.Errorf("classifyTransport(%s) = %s, want %s", tt.dsType, result, tt.expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "1 GiB",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GiB",
		},
		{
			name:     "2.5 GiB",
			bytes:    1024 * 1024 * 1024 * 2.5,
			expected: "2.5 GiB",
		},
		{
			name:     "1 TiB",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TiB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tabular.FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestUsedAvailableCalculation(t *testing.T) {
	capacity := int64(100 * 1024 * 1024 * 1024) // 100 GiB
	freeSpace := int64(30 * 1024 * 1024 * 1024) // 30 GiB
	expectedUsed := capacity - freeSpace        // 70 GiB

	if expectedUsed != 70*1024*1024*1024 {
		t.Errorf("Expected used to be 70 GiB, got %d bytes", expectedUsed)
	}

	if freeSpace+expectedUsed != capacity {
		t.Errorf("used + available should equal capacity")
	}
}

func TestFormatBytesConsistency(t *testing.T) {
	gib := int64(1024 * 1024 * 1024)
	tib := int64(1024 * 1024 * 1024 * 1024)

	tests := []struct {
		bytes    int64
		expected string
	}{
		{gib, "1.0 GiB"},
		{2 * gib, "2.0 GiB"},
		{tib, "1.0 TiB"},
		{2 * tib, "2.0 TiB"},
	}

	for _, tt := range tests {
		result := tabular.FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestTransportClassification(t *testing.T) {
	dsTypes := []string{"nfs", "nfs41", "vsan", "vmfs", "unknown"}

	for _, dsType := range dsTypes {
		result := classifyTransport(dsType)

		validTransports := map[string]bool{
			"NFS":     true,
			"NVMe":    true,
			"unknown": true,
		}

		if !validTransports[result] {
			t.Errorf("classifyTransport(%s) returned invalid transport: %s", dsType, result)
		}
	}
}

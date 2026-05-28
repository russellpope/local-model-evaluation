package model

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "zero bytes",
			bytes:    0,
			expected: "0.0 GiB",
		},
		{
			name:     "1 GiB",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GiB",
		},
		{
			name:     "2.5 GiB",
			bytes:    2 * 1024 * 1024 * 1024 + 512*1024*1024,
			expected: "2.5 GiB",
		},
		{
			name:     "1 TiB",
			bytes:    1024 * 1024 * 1024 * 1024,
			expected: "1.0 TiB",
		},
		{
			name:     "1.5 TiB",
			bytes:    1024*1024*1024*1024 + 512*1024*1024*1024,
			expected: "1.5 TiB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestVMInfoRAMGB(t *testing.T) {
	vm := VMInfo{
		RAM: 8 * 1024 * 1024 * 1024,
	}
	
	expected := 8.0
	actual := vm.RAMGB()
	if actual != expected {
		t.Errorf("RAMGB() = %f, want %f", actual, expected)
	}
}

func TestVMInfoStorageHuman(t *testing.T) {
	vm := VMInfo{
		Storage: 100 * 1024 * 1024 * 1024,
	}
	
	result := vm.StorageHuman()
	if result != "100.0 GiB" {
		t.Errorf("StorageHuman() = %q, want %q", result, "100.0 GiB")
	}
}

func TestDatastoreInfoUsedHuman(t *testing.T) {
	ds := DatastoreInfo{
		Used: 50 * 1024 * 1024 * 1024,
	}
	
	result := ds.UsedHuman()
	if result != "50.0 GiB" {
		t.Errorf("UsedHuman() = %q, want %q", result, "50.0 GiB")
	}
}

func TestDatastoreInfoAvailableHuman(t *testing.T) {
	ds := DatastoreInfo{
		Available: 150 * 1024 * 1024 * 1024,
	}
	
	result := ds.AvailableHuman()
	if result != "150.0 GiB" {
		t.Errorf("AvailableHuman() = %q, want %q", result, "150.0 GiB")
	}
}

func TestClassifyTransport(t *testing.T) {
	result := ClassifyTransport(nil)
	if result != "unknown" {
		t.Errorf("ClassifyTransport(nil) = %q, want %q", result, "unknown")
	}
}

func init() {
	_ = formatBytes
}

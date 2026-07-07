package formatter

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero", 0, "0.0 GiB"},
		{"1 GiB", 1073741824, "1.0 GiB"},
		{"1.5 GiB", 1610612736, "1.5 GiB"},
		{"500 GiB", 536870912000, "500.0 GiB"},
		{"1 TiB", 1099511627776, "1.0 TiB"},
		{"2.5 TiB", 2748779069440, "2.5 TiB"},
		{"512 GiB", 549755813888, "512.0 GiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.expected {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.expected)
			}
		})
	}
}

func TestUsedCapacity(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		available int64
		expected  int64
	}{
		{"normal", 1000, 400, 600},
		{"zero used", 1000, 1000, 0},
		{"available exceeds total", 100, 200, 0},
		{"zero total", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UsedCapacity(tt.total, tt.available)
			if got != tt.expected {
				t.Errorf("UsedCapacity(%d, %d) = %d, want %d", tt.total, tt.available, got, tt.expected)
			}
		})
	}
}

package formatter_test

import (
	"testing"

	"vsphere-inventory/internal/formatter"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name   string
		bytes  int64
		expect string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1024, "1.0 KiB"},
		{"megabytes", 1048576, "1.0 MiB"},
		{"gigabytes", 1073741824, "1.0 GiB"},
		{"gigabytes partial", 2147483648, "2.0 GiB"},
		{"gigabytes fractional", 1610612736, "1.5 GiB"},
		{"terabytes", 1099511627776, "1.0 TiB"},
		{"terabytes fractional", 1649267441664, "1.5 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatter.FormatBytes(tt.bytes)
			if got != tt.expect {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.expect)
			}
		})
	}
}

func TestFormatBytesRounded(t *testing.T) {
	tests := []struct {
		name   string
		bytes  int64
		expect string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 1024, "1 KiB"},
		{"megabytes", 5242880, "5 MiB"},
		{"gigabytes", 1073741824, "1.0 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatter.FormatBytesRounded(tt.bytes)
			if got != tt.expect {
				t.Errorf("FormatBytesRounded(%d) = %q, want %q", tt.bytes, got, tt.expect)
			}
		})
	}
}

func TestUsedEqualsCapacityMinusAvailable(t *testing.T) {
	capacity := int64(107374182400) // 100 GiB
	freeSpace := int64(53687091200) // 50 GiB
	used := capacity - freeSpace

	if used != 53687091200 {
		t.Errorf("used = %d, want 53687091200", used)
	}

	if formatter.FormatBytes(used) != "50.0 GiB" {
		t.Errorf("FormatBytes(%d) = %q, want 50.0 GiB", used, formatter.FormatBytes(used))
	}
}

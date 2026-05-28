package tabular

import (
	"bytes"
	"strings"
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
			name:     "less than 1GiB",
			bytes:    536870912, // 0.5 GiB
			expected: "0.5 GiB",
		},
		{
			name:     "exactly 1GiB",
			bytes:    1073741824, // 1 GiB
			expected: "1.0 GiB",
		},
		{
			name:     "2.5GiB",
			bytes:    2684354560, // 2.5 GiB
			expected: "2.5 GiB",
		},
		{
			name:     "exactly 1TiB",
			bytes:    1099511627776, // 1 TiB
			expected: "1.0 TiB",
		},
		{
			name:     "2.5TiB",
			bytes:    2748779069440, // 2.5 TiB
			expected: "2.5 TiB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(&buf)

	f.PrintHeader("NAME", "VCPU", "RAM")
	f.PrintRow("vm1", "4", "8192")
	f.PrintRow("vm2", "8", "16384")

	err := f.Flush()
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[0], "VCPU") || !strings.Contains(lines[0], "RAM") {
		t.Errorf("Header line should contain NAME, VCPU, RAM: %s", lines[0])
	}
}

func TestFormatterWithDifferentSizes(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(&buf)

	f.PrintHeader("NAME", "SIZE")
	f.PrintRow("small", "100B")
	f.PrintRow("large", "1.5 GiB")

	err := f.Flush()
	if err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected non-empty output")
	}
}

func TestFormatBytesEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{
			name:     "negative bytes",
			bytes:    -1024,
			expected: "-0.0 GiB",
		},
		{
			name:     "very large value",
			bytes:    10995116277760, // 10 TiB
			expected: "10.0 TiB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
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
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

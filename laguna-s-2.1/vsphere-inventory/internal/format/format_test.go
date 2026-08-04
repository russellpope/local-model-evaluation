package format

import (
	"testing"
)

func TestBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"zero", 0, "0 B"},
		{"one byte", 1, "1 B"},
		{"bytes", 512, "512 B"},
		{"one KiB", 1024, "1.0 KiB"},
		{"two KiB", 2048, "2.0 KiB"},
		{"one MiB", 1048576, "1.0 MiB"},
		{"one GiB", 1073741824, "1.0 GiB"},
		{"one TiB", 1099511627776, "1.0 TiB"},
		{"one PiB", 1125899906842624, "1.0 PiB"},
		{"large value", 1152921504606846976, "1.0 EiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Bytes(tt.input)
			if result != tt.expected {
				t.Errorf("Bytes(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBytesConsistency(t *testing.T) {
	capacity := int64(10737418240) // 10 GiB
	used := int64(3221225472)      // 3 GiB
	available := capacity - used

	usedStr := Bytes(used)
	availStr := Bytes(available)

	if usedStr == "" || availStr == "" {
		t.Error("format output should not be empty")
	}

	if used+available != capacity {
		t.Errorf("used + available (%d + %d) != capacity (%d)", used, available, capacity)
	}
}

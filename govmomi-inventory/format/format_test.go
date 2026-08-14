package format

import "testing"

func TestHumanReadable(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0.0B"},
		{512, "512.0B"},
		{1023, "1023.0B"},
		{1024, "1.0KiB"},
		{1536, "1.5KiB"},
		{1048576, "1.0MiB"},
		{1073741824, "1.0GiB"},
		{1099511627776, "1.0TiB"},
		{549755813888, "512.0GiB"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := HumanReadable(tt.input)
			if got != tt.expected {
				t.Errorf("HumanReadable(%d) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

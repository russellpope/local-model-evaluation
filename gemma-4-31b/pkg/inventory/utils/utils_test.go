package utils

import (
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{1024 * 1024 * 1024, "1.0GiB"},
		{2 * 1024 * 1024 * 1024, "2.0GiB"},
		{1024 * 1024 * 1024 * 1024, "1.0TiB"},
		{0, "0.0GiB"},
	}

	for _, tc := range tests {
		res := FormatBytes(tc.bytes)
		if res != tc.expected {
			t.Errorf("FormatBytes(%d) = %s; want %s", tc.bytes, res, tc.expected)
		}
	}
}

func TestFormatRAM(t *testing.T) {
	tests := []struct {
		memMB    int32
		expected float64
	}{
		{1024, 1.0},
		{2048, 2.0},
		{512, 0.5},
	}

	for _, tc := range tests {
		res := FormatRAM(tc.memMB)
		if res != tc.expected {
			t.Errorf("FormatRAM(%d) = %f; want %f", tc.memMB, res, tc.expected)
		}
	}
}

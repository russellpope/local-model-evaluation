package format

import (
	"testing"
)

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero", 0, "0.0 MiB"},
		{"1 MiB", 1 * 1024 * 1024, "1.0 MiB"},
		{"1 GiB", GiB, "1.0 GiB"},
		{"1.5 GiB", int64(1.5 * GiB), "1.5 GiB"},
		{"1 TiB", TiB, "1.0 TiB"},
		{"1.5 TiB", int64(1.5 * TiB), "1.5 TiB"},
		{"10 GiB", 10 * GiB, "10.0 GiB"},
		{"100 GiB", 100 * GiB, "100.0 GiB"},
		{"500 GiB", 500 * GiB, "500.0 GiB"},
		{"2 TiB", 2 * TiB, "2.0 TiB"},
		{"3.5 TiB", int64(3.5 * TiB), "3.5 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("HumanBytes(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestHumanBytesFloat(t *testing.T) {
	tests := []struct {
		name     string
		bytes    float64
		expected string
	}{
		{"zero", 0, "0.0 MiB"},
		{"512 MiB", 512 * float64(MiB), "512.0 MiB"},
		{"1 GiB", float64(GiB), "1.0 GiB"},
		{"1 TiB", float64(TiB), "1.0 TiB"},
		{"1.5 TiB", 1.5 * float64(TiB), "1.5 TiB"},
		{"10 GiB", 10 * float64(GiB), "10.0 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HumanBytesFloat(tt.bytes)
			if result != tt.expected {
				t.Errorf("HumanBytesFloat(%f) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestGB(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected float64
	}{
		{"zero", 0, 0.0},
		{"1 GiB", GiB, 1.0},
		{"2 GiB", 2 * GiB, 2.0},
		{"500 MiB", 500 * MiB, 0.48828125},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GB(tt.bytes)
			if result != tt.expected {
				t.Errorf("GB(%d) = %f, want %f", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestUsedEqualsTotalMinusAvailable(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		available int64
		expected  int64
	}{
		{"normal", 100 * GiB, 30 * GiB, 70 * GiB},
		{"full", 100 * GiB, 0, 100 * GiB},
		{"empty", 100 * GiB, 100 * GiB, 0},
		{"zero total", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			used := UsedBytes(tt.total, tt.available)
			if used != tt.expected {
				t.Errorf("UsedBytes(%d, %d) = %d, want %d", tt.total, tt.available, used, tt.expected)
			}
			if used < 0 {
				t.Errorf("used should not be negative: %d", used)
			}
		})
	}
}

package format

import "testing"

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0.0 GiB"},
		{-5, "0.0 GiB"},
		{512 * MiB, "0.5 GiB"},
		{1 * GiB, "1.0 GiB"},
		{100 * GiB, "100.0 GiB"},
		{1023*GiB + GiB/2, "1023.5 GiB"}, // just under the TiB boundary
		{1 * TiB, "1.0 TiB"},             // boundary flips to TiB
		{TiB + TiB/2, "1.5 TiB"},
		{254*TiB + 384*GiB, "254.4 TiB"}, // one-decimal rounding
	}
	for _, tt := range tests {
		if got := HumanBytes(tt.in); got != tt.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMemoryGB(t *testing.T) {
	tests := []struct {
		in   int32
		want string
	}{
		{0, "0.0 GB"},
		{512, "0.5 GB"},
		{1024, "1.0 GB"},
		{8192, "8.0 GB"},
		{6144, "6.0 GB"},
	}
	for _, tt := range tests {
		if got := MemoryGB(tt.in); got != tt.want {
			t.Errorf("MemoryGB(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestUsedMath(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		available int64
		want      int64
	}{
		{"plain", 100, 30, 70},
		{"full free", 100, 100, 0},
		{"no free", 100, 0, 100},
		{"available exceeds total clamps to 0", 100, 120, 0},
	}
	for _, tt := range tests {
		if got := Used(tt.total, tt.available); got != tt.want {
			t.Errorf("%s: Used(%d,%d) = %d, want %d", tt.name, tt.total, tt.available, got, tt.want)
		}
	}
}

func TestHumanBytesConsistency(t *testing.T) {
	total := int64(3584 * GiB) // 3.5 TiB
	free := int64(1024 * GiB)
	used := Used(total, free)
	if got := HumanBytes(used); got != "2.5 TiB" {
		t.Errorf("used rendered %q, want 2.5 TiB", got)
	}
	if got := HumanBytes(free); got != "1.0 TiB" {
		t.Errorf("free rendered %q, want 1.0 TiB", got)
	}
}

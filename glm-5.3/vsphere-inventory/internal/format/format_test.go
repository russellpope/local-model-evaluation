package format

import "testing"

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"half gib", 512 << 20, "0.5 GiB"},
		{"one gib", 1 << 30, "1.0 GiB"},
		{"gib with fraction", (10<<30 + 512<<20), "10.5 GiB"},
		{"just below tib", tib - 1, "1024.0 GiB"},
		{"one tib", tib, "1.0 TiB"},
		{"tib with fraction", tib + (512 << 30), "1.5 TiB"},
		{"large", 3 * tib, "3.0 TiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HumanBytes(tt.in); got != tt.want {
				t.Errorf("HumanBytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestHumanGB(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GB"},
		{"512 MB", 512, "0.5 GB"},
		{"1 GB", 1024, "1.0 GB"},
		{"4 GB", 4096, "4.0 GB"},
		{"fraction", 1536, "1.5 GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HumanGB(tt.in); got != tt.want {
				t.Errorf("HumanGB(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUsedBytes(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		available int64
		want      int64
	}{
		{"all free", 100, 100, 0},
		{"all used", 100, 0, 100},
		{"partial", 100, 40, 60},
		{"zero capacity", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UsedBytes(tt.total, tt.available); got != tt.want {
				t.Errorf("UsedBytes(%d, %d) = %d, want %d", tt.total, tt.available, got, tt.want)
			}
		})
	}
}

package format

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0.0 GiB"},
		{"under one GiB", 512 << 20, "0.5 GiB"},
		{"one GiB", 1 << 30, "1.0 GiB"},
		{"fractional GiB", 1536 << 20, "1.5 GiB"},
		{"large GiB", 900 << 30, "900.0 GiB"},
		{"one TiB", 1 << 40, "1.0 TiB"},
		{"fractional TiB", 1536 << 30, "1.5 TiB"},
		{"multi TiB", 2858730232218, "2.6 TiB"}, // ~2.6 TiB
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBytes(tt.bytes); got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatGB(t *testing.T) {
	tests := []struct {
		name string
		mib  int64
		want string
	}{
		{"zero", 0, "0.0"},
		{"one GB", 1024, "1.0"},
		{"fractional", 2560, "2.5"},
		{"typical VM", 4096, "4.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatGB(tt.mib); got != tt.want {
				t.Errorf("FormatGB(%d) = %q, want %q", tt.mib, got, tt.want)
			}
		})
	}
}

func TestUsedCapacity(t *testing.T) {
	tests := []struct {
		name        string
		total, free int64
		want        int64
	}{
		{"partially used", 100, 40, 60},
		{"empty volume", 100, 100, 0},
		{"free exceeds total clamps to zero", 100, 150, 0},
		{"fully used", 100, 0, 100},
		{"zero capacity", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UsedCapacity(tt.total, tt.free); got != tt.want {
				t.Errorf("UsedCapacity(%d, %d) = %d, want %d", tt.total, tt.free, got, tt.want)
			}
		})
	}
}

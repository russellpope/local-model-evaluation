package inventory

import (
	"testing"
)

const mib = 1 << 20

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"half GiB", int64(512 * mib), "0.5 GiB"},
		{"one GiB", gib, "1.0 GiB"},
		{"one and a half GiB", gib + int64(512*mib), "1.5 GiB"},
		{"just under TiB", tib - 1, "1024.0 GiB"},
		{"one TiB", tib, "1.0 TiB"},
		{"one and a half TiB", tib + gib/2*1024, "1.5 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBytes(tt.in); got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUsedOf(t *testing.T) {
	tests := []struct {
		name           string
		capacity, free int64
		want           int64
	}{
		{"empty datastore", 100, 100, 0},
		{"partially used", 100, 40, 60},
		{"fully used", 100, 0, 100},
		{"free exceeds capacity clamps to zero", 100, 200, 0},
		{"negative free clamps to zero", 100, -1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UsedOf(tt.capacity, tt.free); got != tt.want {
				t.Errorf("UsedOf(%d, %d) = %d, want %d", tt.capacity, tt.free, got, tt.want)
			}
		})
	}
}

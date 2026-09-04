package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"one GiB", 1 << 30, "1.0 GiB"},
		{"five GiB", 5 * (1 << 30), "5.0 GiB"},
		{"negative clamps to zero", -42, "0.0 GiB"},
		{"just under 1 TiB stays GiB", (1 << 40) - 1, "1024.0 GiB"},
		{"one TiB", 1 << 40, "1.0 TiB"},
		{"two and a half TiB", 2*(1<<40) + 512*(1<<30), "2.5 TiB"},
		{"ten TiB", 10 * (1 << 40), "10.0 TiB"},
		{"fractional GiB rounds to one decimal", 3*(1<<30) + 7*(1<<30)/10, "3.7 GiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Bytes(tt.in); got != tt.want {
				t.Errorf("Bytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUsed(t *testing.T) {
	tests := []struct {
		name     string
		capacity int64
		free     int64
		want     int64
	}{
		{"plain subtraction", 100, 30, 70},
		{"full", 100, 0, 100},
		{"empty", 100, 100, 0},
		{"free exceeds capacity clamps", 100, 120, 0},
		{"negative free clamps", 100, -5, 100},
		{"negative capacity clamps", -10, -5, 0},
		{"zero capacity", 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Used(tt.capacity, tt.free); got != tt.want {
				t.Errorf("Used(%d, %d) = %d, want %d", tt.capacity, tt.free, got, tt.want)
			}
		})
	}
}

func TestMemoryMB(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{8192, "8 GB"},
		{1536, "1.5 GB"},
		{32768, "32 GB"},
		{0, "0 GB"},
	}
	for _, tt := range tests {
		if got := MemoryMB(tt.in); got != tt.want {
			t.Errorf("MemoryMB(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0.0 GiB"},
		{"negative clamps to zero", -42, "0.0 GiB"},
		{"half GiB", 512 * (1 << 20), "0.5 GiB"},
		{"one GiB", 1 << 30, "1.0 GiB"},
		{"one and a half GiB", 1536 * (1 << 20), "1.5 GiB"},
		{"just under one TiB", (1 << 40) - 1, "1024.0 GiB"},
		{"one TiB", 1 << 40, "1.0 TiB"},
		{"two and a half TiB", 5 * (1 << 40) / 2, "2.5 TiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Bytes(tt.bytes); got != tt.want {
				t.Errorf("Bytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestRAMMB(t *testing.T) {
	tests := []struct {
		miB  int64
		want string
	}{
		{0, "0.0 GB"},
		{512, "0.5 GB"},
		{1024, "1.0 GB"},
		{4096, "4.0 GB"},
		{6144, "6.0 GB"},
	}
	for _, tt := range tests {
		if got := RAMMB(tt.miB); got != tt.want {
			t.Errorf("RAMMB(%d) = %q, want %q", tt.miB, got, tt.want)
		}
	}
}

func TestUsed(t *testing.T) {
	tests := []struct {
		name  string
		total int64
		free  int64
		want  int64
	}{
		{"typical", 100, 40, 60},
		{"full", 100, 0, 100},
		{"empty", 100, 100, 0},
		{"inconsistent values clamp to zero", 100, 120, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Used(tt.total, tt.free); got != tt.want {
				t.Errorf("Used(%d, %d) = %d, want %d", tt.total, tt.free, got, tt.want)
			}
		})
	}
}

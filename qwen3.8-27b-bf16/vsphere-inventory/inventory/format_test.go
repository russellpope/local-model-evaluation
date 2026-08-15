package inventory

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{-1, "unknown"},
		{0, "0.0GiB"},
		{1 << 20, "0.0GiB"}, // 1 MiB rounds to 0.0 GiB at one decimal
		{512 << 20, "0.5GiB"},
		{1 << 30, "1.0GiB"},
		{3*(1<<30) + (1 << 29), "3.5GiB"},
		{1023<<30 + (1 << 29), "1023.5GiB"}, // just under 1 TiB stays in GiB
		{1 << 40, "1.0TiB"},
		{2560 << 30, "2.5TiB"},
	}
	for _, tc := range tests {
		if got := FormatBytes(tc.in); got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestUsedBytes(t *testing.T) {
	tests := []struct {
		name      string
		total     int64
		available int64
		want      int64
	}{
		{"normal", 100, 30, 70},
		{"fully used", 100, 0, 100},
		{"fully available", 100, 100, 0},
		{"clamped", 100, 120, 0},
		{"empty", 0, 0, 0},
		{"gib scale", 5 << 30, 2 << 30, 3 << 30},
	}
	for _, tc := range tests {
		if got := UsedBytes(tc.total, tc.available); got != tc.want {
			t.Errorf("%s: UsedBytes(%d, %d) = %d, want %d", tc.name, tc.total, tc.available, got, tc.want)
		}
	}
}

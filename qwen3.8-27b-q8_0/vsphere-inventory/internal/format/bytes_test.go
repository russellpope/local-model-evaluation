package format

import "testing"

func TestFormatBytes(t *testing.T) {
	const (
		mib = int64(1) << 20
		gib = int64(1) << 30
		tib = int64(1) << 40
	)

	tests := []struct {
		in   int64
		want string
	}{
		{0, "0.0 GiB"},
		{512 * mib, "0.5 GiB"},
		{gib, "1.0 GiB"},
		{1536 * mib, "1.5 GiB"},
		{512 * gib, "512.0 GiB"},
		{tib - gib, "1023.0 GiB"},
		{tib, "1.0 TiB"},
		{1536 * gib, "1.5 TiB"},
		{2 * tib, "2.0 TiB"},
		{-42, "0.0 GiB"},
	}

	for _, tt := range tests {
		if got := FormatBytes(tt.in); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestUsedBytes(t *testing.T) {
	const gib = int64(1) << 30

	tests := []struct {
		total int64
		avail int64
		want  int64
	}{
		{10 * gib, 4 * gib, 6 * gib},
		{10 * gib, 10 * gib, 0},
		{10 * gib, 0, 10 * gib},
		{0, 0, 0},
		{10 * gib, 12 * gib, 0}, // available > total clamps to 0
		{0, 5 * gib, 0},         // no capacity, free space reported as 0
		{4*gib + 1, 4 * gib, 1}, // sub-GiB precision preserved
	}

	for _, tt := range tests {
		if got := UsedBytes(tt.total, tt.avail); got != tt.want {
			t.Errorf("UsedBytes(%d, %d) = %d, want %d", tt.total, tt.avail, got, tt.want)
		}
	}
}

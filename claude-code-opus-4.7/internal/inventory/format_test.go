package inventory

import "testing"

func TestFormatBytes(t *testing.T) {
	const (
		mib = int64(1) << 20
		gib = int64(1) << 30
		tib = int64(1) << 40
	)
	cases := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0.0 GiB"},
		{"half gib", 512 * mib, "0.5 GiB"},
		{"one gib", gib, "1.0 GiB"},
		{"one and a half gib", gib + 512*mib, "1.5 GiB"},
		{"just under one tib stays gib", 1023 * gib, "1023.0 GiB"},
		{"one tib", tib, "1.0 TiB"},
		{"two and a half tib", 2*tib + 512*gib, "2.5 TiB"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := FormatBytes(tc.bytes)
			if got != tc.want {
				t.Fatalf("FormatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestDatastoreUsedBytes(t *testing.T) {
	const tib = int64(1) << 40
	cases := []struct {
		name     string
		capacity int64
		free     int64
		want     int64
	}{
		{"simple", 100, 30, 70},
		{"empty", 0, 0, 0},
		{"full", 50, 0, 50},
		{"large", 5 * tib, 2 * tib, 3 * tib},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := DatastoreInfo{CapacityBytes: tc.capacity, FreeBytes: tc.free}
			if got := d.UsedBytes(); got != tc.want {
				t.Fatalf("UsedBytes(cap=%d free=%d) = %d, want %d", tc.capacity, tc.free, got, tc.want)
			}
		})
	}
}

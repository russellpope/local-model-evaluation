package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"exactly 1 GiB", 1 << 30, "1.0 GiB"},
		{"half GiB", 1 << 29, "0.5 GiB"},
		{"10 GiB", 10 << 30, "10.0 GiB"},
		{"1.5 GiB", 1536 << 20, "1.5 GiB"},
		{"just under 1 TiB stays GiB", (1 << 40) - 1, "1024.0 GiB"},
		{"exactly 1 TiB", 1 << 40, "1.0 TiB"},
		{"1.5 TiB", 1536 << 30, "1.5 TiB"},
		{"4 TiB", 4 << 40, "4.0 TiB"},
		{"small bytes round to 0.0 GiB", 4096, "0.0 GiB"},
		{"VM committed 234 bytes", 234, "0.0 GiB"},
		{"negative clamps", -5, "0.0 GiB"},
		{"gib rounding one decimal", 1500 << 20, "1.5 GiB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Bytes(tc.in); got != tc.want {
				t.Errorf("Bytes(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMemoryGB(t *testing.T) {
	tests := []struct {
		name string
		mb   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"1024 MiB is 1 GiB", 1024, "1.0 GiB"},
		{"4096 MiB", 4096, "4.0 GiB"},
		{"half", 512, "0.5 GiB"},
		{"32 MiB rounds down", 32, "0.0 GiB"},
		{"negative clamps", -1, "0.0 GiB"},
		{"16 GiB", 16384, "16.0 GiB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := MemoryGB(tc.mb); got != tc.want {
				t.Errorf("MemoryGB(%d) = %q, want %q", tc.mb, got, tc.want)
			}
		})
	}
}

func TestUsed(t *testing.T) {
	gib := int64(1 << 30)
	tests := []struct {
		name        string
		total, free int64
		want        int64
	}{
		{"normal", 100 * gib, 40 * gib, 60 * gib},
		{"zero total", 0, 0, 0},
		{"negative total", -10, 0, 0},
		{"negative free clamps", 100 * gib, -5, 100 * gib},
		{"free exceeds total clamps used to zero", 100 * gib, 200 * gib, 0},
		{"fully used", 100 * gib, 0, 100 * gib},
		{"fully free", 100 * gib, 100 * gib, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Used(tc.total, tc.free); got != tc.want {
				t.Errorf("Used(%d, %d) = %d, want %d", tc.total, tc.free, got, tc.want)
			}
		})
	}
}

// TestUsedReconstructsCapacity is the property test behind the datastore
// consistency check: used + available == capacity for every sane input.
func TestUsedReconstructsCapacity(t *testing.T) {
	gib := int64(1 << 30)
	for _, cap := range []int64{gib, 100 * gib, 4 << 40, 1 << 40} {
		for _, pct := range []int{0, 25, 50, 99, 100} {
			free := cap * int64(pct) / 100
			if Used(cap, free)+free != cap {
				t.Errorf("capacity %d, free %d%%: used+free != capacity", cap, pct)
			}
		}
	}
}

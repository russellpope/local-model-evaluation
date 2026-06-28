package inventory

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"one_byte", 1, "0.0 GiB"},
		{"small_gib", 500 * 1024 * 1024, "0.5 GiB"},
		{"exact_gib", 1 << 30, "1.0 GiB"},
		{"two_point_five_gib", int64(2.5 * float64(1<<30)), "2.5 GiB"},
		{"one_tib", 1 << 40, "1.0 TiB"},
		{"above_tib", 1<<40 + 500*1024*1024*1024, "1.5 TiB"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.in)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUsedFromCapacity(t *testing.T) {
	tests := []struct {
		name      string
		capacity  int64
		available int64
		want      int64
	}{
		{"normal", 100, 30, 70},
		{"empty", 100, 100, 0},
		{"full", 100, 0, 100},
		{"negative_available_clamps_to_zero", 100, 200, 0},
		{"used_exceeds_capacity_clamps", 100, -50, 100},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := UsedFromCapacity(tt.capacity, tt.available)
			if got != tt.want {
				t.Errorf("UsedFromCapacity(%d,%d) = %d, want %d", tt.capacity, tt.available, got, tt.want)
			}
		})
	}

	// Consistency: used + available should never exceed capacity.
	got := UsedFromCapacity(1000, 300)
	if got+300 > 1000 {
		t.Errorf("UsedFromCapacity(1000,300)= %d, but used+available (%d+300=%d) exceeds capacity", got, got, got+300)
	}
}

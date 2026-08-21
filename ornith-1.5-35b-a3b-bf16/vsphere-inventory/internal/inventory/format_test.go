package inventory

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"one_kib", 1024, "1.0 KiB"},
		{"one_mib", 1024 * 1024, "1.0 MiB"},
		{"one_gib", 1024 * 1024 * 1024, "1.0 GiB"},
		{"one_and_half_gib", 1610612736, "1.5 GiB"},
		{"ten_gib", 10737418240, "10.0 GiB"},
		{"one_tib", 1024 * 1024 * 1024 * 1024, "1.0 TiB"},
		{"two_and_half_tib", 2748779069440, "2.5 TiB"},
		{"negative_clamped", -100, "0 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBytes(tt.bytes); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
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
		{"normal", 100, 30, 70},
		{"empty", 100, 100, 0},
		{"full_capacity_zero_free", 500, 0, 500},
		{"available_exceeds_capacity_clamped", 100, 150, 0},
		{"zero", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := usedBytes(tt.total, tt.available); got != tt.want {
				t.Errorf("usedBytes(%d, %d) = %d, want %d", tt.total, tt.available, got, tt.want)
			}
		})
	}
}

func TestFormatGB(t *testing.T) {
	tests := []struct {
		name string
		mb   int64
		want string
	}{
		{"1_gib", 1024, "1.0 GB"},
		{"8_gib", 8192, "8.0 GB"},
		{"3_gib_half", 3072, "3.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatGB(gbFromMB(tt.mb))
			if got != tt.want {
				t.Errorf("formatGB(gbFromMB(%d)) = %q, want %q", tt.mb, got, tt.want)
			}
		})
	}
}

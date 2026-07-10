package main

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{name: "zero", in: 0, want: "0.0 GiB"},
		{name: "one gib", in: 1 << 30, want: "1.0 GiB"},
		{name: "one and half gib", in: int64(1.5 * (1 << 30)), want: "1.5 GiB"},
		{name: "two tib", in: 2 << 40, want: "2.0 TiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatBytes(tt.in); got != tt.want {
				t.Fatalf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUsedBytes(t *testing.T) {
	got := UsedBytes(10<<30, 3<<30)
	if got != 7<<30 {
		t.Fatalf("UsedBytes = %d, want %d", got, int64(7<<30))
	}
	if got := UsedBytes(3<<30, 10<<30); got != 0 {
		t.Fatalf("UsedBytes should not go negative, got %d", got)
	}
}

package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"one GiB", GiB, "1.0 GiB"},
		{"one point five GiB", GiB + 512*MiB, "1.5 GiB"},
		{"exactly one TiB", TiB, "1.0 TiB"},
		{"two TiB", 2 * TiB, "2.0 TiB"},
		{"hundreds of GiB", 150 * GiB, "150.0 GiB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Bytes(tt.in); got != tt.want {
				t.Errorf("Bytes(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGigabytesFromMiB(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0"},
		{"4 GiB configured", 4096, "4.0"},
		{"1 GiB", 1024, "1.0"},
		{"half GiB", 512, "0.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GigabytesFromMiB(tt.in); got != tt.want {
				t.Errorf("GigabytesFromMiB(%d) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

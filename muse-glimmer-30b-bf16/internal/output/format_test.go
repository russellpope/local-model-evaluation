package output

import "testing"

func TestBytesToHuman(t *testing.T) {
	tests := []struct {
		in  int64
		out string
	}{
		{0, "0 GiB"},
		{1024 * 1024 * 1024, "1.0 GiB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TiB"},
		{1024 * 1024 * 1024 * 2048, "2.0 TiB"},
	}
	for _, tt := range tests {
		if got := BytesToHuman(tt.in); got != tt.out {
			t.Errorf("BytesToHuman(%d) = %s, want %s", tt.in, got, tt.out)
		}
	}
}

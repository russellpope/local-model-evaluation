package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"zero", 0, "0.0 GiB"},
		{"negative floors", -5, "0.0 GiB"},
		{"one GiB", 1 << 30, "1.0 GiB"},
		{"half GiB", 1 << 29, "0.5 GiB"},
		{"10.5 GiB", 10*GiBtest + 512*MiBtest, "10.5 GiB"},
		{"just under TiB", TiBtest - 1, "1024.0 GiB"},
		{"exactly TiB", TiBtest, "1.0 TiB"},
		{"1.5 TiB", TiBtest + 512*GiBtest, "1.5 TiB"},
		{"20 TiB", 20 * TiBtest, "20.0 TiB"},
		{"rounds to tenth", 1*GiBtest + 64*MiBtest, "1.1 GiB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Bytes(tc.in); got != tc.want {
				t.Errorf("Bytes(%d) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

const (
	MiBtest = 1 << 20
	GiBtest = 1 << 30
	TiBtest = 1 << 40
)

func TestGB(t *testing.T) {
	tests := []struct {
		miB  int64
		want string
	}{
		{0, "0.0 GiB"},
		{1024, "1.0 GiB"},
		{4096, "4.0 GiB"},
		{512, "0.5 GiB"},
		{65536, "64.0 GiB"},
		{2097152, "2.0 TiB"}, // 2 TiB expressed in MiB
	}
	for _, tc := range tests {
		if got := GB(tc.miB); got != tc.want {
			t.Errorf("GB(%d MiB) = %q, want %q", tc.miB, got, tc.want)
		}
	}
}

func TestUsed(t *testing.T) {
	tests := []struct {
		name           string
		capacity, free int64
		want           int64
	}{
		{"subtraction", 100 * GiBtest, 40 * GiBtest, 60 * GiBtest},
		{"full", 100 * GiBtest, 0, 100 * GiBtest},
		{"empty", 100 * GiBtest, 100 * GiBtest, 0},
		{"free exceeds capacity floors to zero", 100, 150, 0},
		{"zero capacity", 0, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Used(tc.capacity, tc.free); got != tc.want {
				t.Errorf("Used(%d, %d) = %d, want %d", tc.capacity, tc.free, got, tc.want)
			}
		})
	}
}

// The used/available math must stay consistent with what the table renderer
// prints: used + available == capacity for any sane summary.
func TestUsedAvailableConsistency(t *testing.T) {
	total, free := int64(2*TiBtest+500*GiBtest), int64(1*TiBtest)
	used := Used(total, free)
	if used+free != total {
		t.Fatalf("used+free = %d, capacity = %d", used+free, total)
	}
	// used = 1 TiB + 500 GiB = 1.49 TiB → rounds to 1.5
	if got := Bytes(used); got != "1.5 TiB" {
		t.Errorf("Bytes(used) = %q, want 1.5 TiB", got)
	}
	if got := Bytes(free); got != "1.0 TiB" {
		t.Errorf("Bytes(free) = %q, want 1.0 TiB", got)
	}
}

package client

import (
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"full SDK URL", "https://vc.lab/sdk", "https://vc.lab/sdk", false},
		{"bare host gets scheme and sdk", "vc.lab", "https://vc.lab/sdk", false},
		{"host with port", "https://127.0.0.1:8989/sdk", "https://127.0.0.1:8989/sdk", false},
		{"host:port without scheme", "127.0.0.1:8989", "https://127.0.0.1:8989/sdk", false},
		{"root path becomes sdk", "https://vc.lab/", "https://vc.lab/sdk", false},
		{"custom path kept", "https://vc.lab/inventory/service", "https://vc.lab/inventory/service/sdk", false},
		{"already sdk-prefixed path kept", "https://vc.lab/sdk/vimService", "https://vc.lab/sdk/vimService", false},
		{"empty", "", "", true},
		{"whitespace", "   ", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := ParseURL(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseURL(%q) = %v, want error", tt.in, u)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseURL(%q): %v", tt.in, err)
			}
			if got := u.String(); got != tt.want {
				t.Errorf("ParseURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

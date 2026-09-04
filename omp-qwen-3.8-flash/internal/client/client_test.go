package client

import "testing"

func TestParseURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  bool
	}{
		{"127.0.0.1:8989", "https://127.0.0.1:8989/sdk", false},
		{"vc.lab", "https://vc.lab/sdk", false},
		{"https://vc.lab", "https://vc.lab/sdk", false},
		{"https://vc.lab/sdk", "https://vc.lab/sdk", false},
		{"http://127.0.0.1:8989", "http://127.0.0.1:8989/sdk", false},
		{"https://user:pw@vc.lab/sdk", "https://vc.lab/sdk", false},
		{"https://vc.lab/sdk/", "https://vc.lab/sdk/", false},
		{"//", "", true},
	}
	for _, tc := range tests {
		got, err := ParseURL(tc.in)
		if tc.err {
			if err == nil {
				t.Errorf("ParseURL(%q): want error, got %v", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseURL(%q): %v", tc.in, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("ParseURL(%q) = %q, want %q", tc.in, got.String(), tc.want)
		}
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func TestLoadPrecedence(t *testing.T) {
	file := writeConfigFile(t, "url: https://file-vc.lab/sdk\nusername: file-user\ntimeout: 90s\n")

	tests := []struct {
		name    string
		env     map[string]string
		flags   FlagValues
		useFile bool
		want    Config
		wantErr bool
	}{
		{
			name:    "defaults only is rejected: URL required",
			flags:   FlagValues{},
			wantErr: true,
		},
		{
			// url comes from the file; timeout/insecure prove the defaults.
			name:    "config file beats defaults",
			useFile: true,
			want: Config{
				URL:      "https://file-vc.lab/sdk",
				Username: "file-user",
				Insecure: false,
				Timeout:  90 * time.Second,
			},
		},
		{
			name:    "env var beats config file",
			env:     map[string]string{"VSPHERE_URL": "https://env-vc.lab/sdk"},
			flags:   FlagValues{},
			useFile: true,
			want: Config{
				URL:      "https://env-vc.lab/sdk",
				Username: "file-user",
				Timeout:  90 * time.Second,
			},
		},
		{
			name:    "flag beats env var and config file",
			env:     map[string]string{"VSPHERE_URL": "https://env-vc.lab/sdk"},
			flags:   FlagValues{"url": "https://flag-vc.lab/sdk"},
			useFile: true,
			want: Config{
				URL:      "https://flag-vc.lab/sdk",
				Username: "file-user",
				Timeout:  90 * time.Second,
			},
		},
		{
			name: "bool env var parses",
			env:  map[string]string{"VSPHERE_URL": "https://x.lab/sdk", "VSPHERE_INSECURE": "true", "VSPHERE_PASSWORD": "p"},
			want: Config{
				URL:      "https://x.lab/sdk",
				Password: "p",
				Insecure: true,
				Timeout:  60 * time.Second,
			},
		},
		{
			name:  "explicit false flag overrides true env var",
			env:   map[string]string{"VSPHERE_URL": "https://x.lab/sdk", "VSPHERE_INSECURE": "true"},
			flags: FlagValues{"insecure": "false"},
			want:  Config{URL: "https://x.lab/sdk", Insecure: false, Timeout: 60 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfgFile := ""
			if tt.useFile {
				cfgFile = file
			}
			got, err := Load(tt.flags, cfgFile)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if got.URL != tt.want.URL {
				t.Errorf("URL = %q, want %q", got.URL, tt.want.URL)
			}
			if got.Username != tt.want.Username {
				t.Errorf("Username = %q, want %q", got.Username, tt.want.Username)
			}
			if got.Password != tt.want.Password {
				t.Errorf("Password = %q, want %q", got.Password, tt.want.Password)
			}
			if got.Insecure != tt.want.Insecure {
				t.Errorf("Insecure = %t, want %t", got.Insecure, tt.want.Insecure)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %s, want %s", got.Timeout, tt.want.Timeout)
			}
		})
	}
}

func TestLoadMissingURLErrors(t *testing.T) {
	_, err := Load(FlagValues{}, "")
	if err == nil {
		t.Fatal("Load() with no URL configured: expected error, got nil")
	}
}

func TestLoadBadConfigFileErrors(t *testing.T) {
	_, err := Load(FlagValues{}, filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("Load() with missing config file: expected error, got nil")
	}
}

func TestLoadInvalidTimeoutErrors(t *testing.T) {
	_, err := Load(FlagValues{"url": "https://x.lab/sdk", "timeout": "-5s"}, "")
	if err == nil {
		t.Fatal("Load() with negative timeout: expected error, got nil")
	}
}

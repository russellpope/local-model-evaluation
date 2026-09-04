package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

// TestPrecedence asserts flag > env > file > default, using the exact
// resolution path the CLI walks (New -> ReadConfig -> SetOverride).
func TestPrecedence(t *testing.T) {
	file := writeConfigFile(t, "url: https://from-file/sdk\ntimeout: 10s\ninsecure: true\n")

	// Tier 4: built-in defaults with no file and no env.
	c := New()
	s, err := c.Settings()
	if err != nil {
		t.Fatalf("defaults: Settings: %v", err)
	}
	if s.Timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", s.Timeout)
	}
	if s.Insecure {
		t.Error("default insecure = true, want false")
	}
	if s.URL != "" {
		t.Errorf("default url = %q, want empty", s.URL)
	}

	// Tier 3: config file overrides defaults.
	if err := c.ReadConfig(file); err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	s, err = c.Settings()
	if err != nil {
		t.Fatalf("file: Settings: %v", err)
	}
	if s.Timeout != 10*time.Second {
		t.Errorf("file timeout = %v, want 10s", s.Timeout)
	}
	if !s.Insecure {
		t.Error("file insecure = false, want true")
	}
	if s.URL != "https://from-file/sdk" {
		t.Errorf("file url = %q, want https://from-file/sdk", s.URL)
	}

	// Tier 2: environment variables override the config file.
	t.Setenv(EnvPrefix+"_TIMEOUT", "20s")
	t.Setenv(EnvPrefix+"_URL", "https://from-env/sdk")
	s, err = c.Settings()
	if err != nil {
		t.Fatalf("env: Settings: %v", err)
	}
	if s.Timeout != 20*time.Second {
		t.Errorf("env timeout = %v, want 20s", s.Timeout)
	}
	if s.URL != "https://from-env/sdk" {
		t.Errorf("env url = %q, want https://from-env/sdk", s.URL)
	}

	// Tier 1: an explicitly set flag overrides env and file.
	c.SetOverride("timeout", "30s")
	c.SetOverride("url", "https://from-flag/sdk")
	s, err = c.Settings()
	if err != nil {
		t.Fatalf("flag: Settings: %v", err)
	}
	if s.Timeout != 30*time.Second {
		t.Errorf("flag timeout = %v, want 30s", s.Timeout)
	}
	if s.URL != "https://from-flag/sdk" {
		t.Errorf("flag url = %q, want https://from-flag/sdk", s.URL)
	}
}

func TestReadConfigMissingFile(t *testing.T) {
	c := New()
	if err := c.ReadConfig(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("ReadConfig of missing file = nil error, want error")
	}
	if err := c.ReadConfig(""); err != nil {
		t.Fatalf("ReadConfig(\"\") = %v, want nil", err)
	}
}

func TestInvalidTimeoutRejected(t *testing.T) {
	c := New()
	c.SetOverride("timeout", "0s")
	if _, err := c.Settings(); err == nil {
		t.Fatal("Settings with zero timeout = nil error, want error")
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// newFlags returns a flag set mirroring the shared connection flags on the
// root command, so a test can flip individual flags to "given on the CLI".
func newFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("url", "", "")
	fs.String("username", "", "")
	fs.String("password", "", "")
	fs.Bool("insecure", false, "")
	fs.Duration("timeout", 0, "")
	return fs
}

func writeYAML(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("VSPHERE_URL", "")
	cfg, err := Load(nil, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "" {
		t.Errorf("default url = %q, want empty", cfg.URL)
	}
	if cfg.Insecure {
		t.Errorf("default insecure = true, want false")
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("default timeout = %v, want %v", cfg.Timeout, DefaultTimeout)
	}
}

func TestLoadConfigFileProvidesURL(t *testing.T) {
	t.Setenv("VSPHERE_URL", "")
	p := writeYAML(t, "url: https://vc.lab/sdk\nusername: file-user\n")
	cfg, err := Load(nil, p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "https://vc.lab/sdk" {
		t.Errorf("url = %q, want file value", cfg.URL)
	}
	if cfg.Username != "file-user" {
		t.Errorf("username = %q, want file value", cfg.Username)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	p := writeYAML(t, "url: file-url\ntimeout: 45s\n")
	t.Setenv("VSPHERE_URL", "env-url")
	t.Setenv("VSPHERE_TIMEOUT", "90s")

	cfg, err := Load(nil, p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "env-url" {
		t.Errorf("url = %q, want env value", cfg.URL)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("timeout = %v, want env value 90s", cfg.Timeout)
	}
}

func TestFlagOverridesEnv(t *testing.T) {
	p := writeYAML(t, "url: file-url\nusername: file-user\n")
	t.Setenv("VSPHERE_URL", "env-url")
	t.Setenv("VSPHERE_USERNAME", "env-user")
	t.Setenv("VSPHERE_PASSWORD", "env-pass")

	fs := newFlags()
	if err := fs.Set("url", "flag-url"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(fs, p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "flag-url" {
		t.Errorf("url = %q, want flag value", cfg.URL)
	}
	if cfg.Username != "env-user" {
		t.Errorf("username = %q, want env value", cfg.Username)
	}
	if cfg.Password != "env-pass" {
		t.Errorf("password = %q, want env value", cfg.Password)
	}
}

// TestLoadFullPrecedence exercises all four layers at once, asserting
// flag > env > file > default for each field.
func TestLoadFullPrecedence(t *testing.T) {
	p := writeYAML(t, "url: file-url\nusername: file-user\npassword: file-pass\ninsecure: true\ntimeout: 60s\n")
	t.Setenv("VSPHERE_URL", "env-url")
	t.Setenv("VSPHERE_USERNAME", "env-user")
	t.Setenv("VSPHERE_PASSWORD", "env-pass")
	t.Setenv("VSPHERE_INSECURE", "false")
	t.Setenv("VSPHERE_TIMEOUT", "90s")

	fs := newFlags()
	for name, val := range map[string]string{
		"url": "flag-url", "username": "flag-user", "password": "flag-pass",
	} {
		if err := fs.Set(name, val); err != nil {
			t.Fatal(err)
		}
	}
	if err := fs.Set("insecure", "true"); err != nil {
		t.Fatal(err)
	}
	if err := fs.Set("timeout", "2m"); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(fs, p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.URL != "flag-url" {
		t.Errorf("url = %q, want flag (highest precedence)", cfg.URL)
	}
	if cfg.Username != "flag-user" {
		t.Errorf("username = %q, want flag", cfg.Username)
	}
	if cfg.Password != "flag-pass" {
		t.Errorf("password = %q, want flag", cfg.Password)
	}
	if !cfg.Insecure {
		t.Errorf("insecure = false, want flag true overriding env false and file true")
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("timeout = %v, want flag 2m", cfg.Timeout)
	}
}

func TestEnvOverridesFlag_NoFlagGiven(t *testing.T) {
	// No changed flags, so env must win over the file for every field.
	p := writeYAML(t, "url: file-url\nusername: file-user\npassword: file-pass\ntimeout: 60s\n")
	t.Setenv("VSPHERE_URL", "env-url")
	t.Setenv("VSPHERE_USERNAME", "env-user")
	t.Setenv("VSPHERE_PASSWORD", "env-pass")

	cfg, err := Load(newFlags(), p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "env-url" || cfg.Username != "env-user" || cfg.Password != "env-pass" {
		t.Errorf("expected all-env resolution, got %+v", cfg)
	}
}

func TestLoadBadTimeoutReturnsError(t *testing.T) {
	p := writeYAML(t, "url: https://vc.lab/sdk\ntimeout: banana\n")
	if cfg, err := Load(nil, p); err == nil {
		t.Errorf("expected error for invalid timeout, got %+v", cfg)
	}
}

func TestLoadMissingConfigFileReturnsError(t *testing.T) {
	if _, err := Load(nil, filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Errorf("expected error for missing config file")
	}
}

func TestLoadBareSecondTimeout(t *testing.T) {
	p := writeYAML(t, "url: https://vc.lab/sdk\ntimeout: 30\n")
	cfg, err := Load(nil, p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", cfg.Timeout)
	}
}

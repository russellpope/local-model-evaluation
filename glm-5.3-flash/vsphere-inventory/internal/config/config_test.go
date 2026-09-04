package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func newFlags(t *testing.T) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddFlags(fs)
	return fs
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func TestConfigPrecedence(t *testing.T) {
	const fileYAML = `
url: https://file-vc.lab/sdk
username: file-user
timeout: 30s
insecure: true
`

	t.Run("default when only env provides url", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://env-only.lab/sdk")
		cfg, err := Load(newFlags(t))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.URL != "https://env-only.lab/sdk" {
			t.Errorf("URL = %q, want env value", cfg.URL)
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("Timeout = %s, want default 60s", cfg.Timeout)
		}
		if cfg.Insecure {
			t.Errorf("Insecure = true, want default false")
		}
	})

	t.Run("config file overrides default", func(t *testing.T) {
		fs := newFlags(t)
		if err := fs.Set("config", writeConfigFile(t, fileYAML)); err != nil {
			t.Fatalf("set --config: %v", err)
		}
		cfg, err := Load(fs)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.URL != "https://file-vc.lab/sdk" {
			t.Errorf("URL = %q, want file value", cfg.URL)
		}
		if cfg.Timeout.String() != "30s" {
			t.Errorf("Timeout = %s, want file value 30s", cfg.Timeout)
		}
		if !cfg.Insecure {
			t.Errorf("Insecure = false, want file value true")
		}
	})

	t.Run("env overrides config file", func(t *testing.T) {
		fs := newFlags(t)
		if err := fs.Set("config", writeConfigFile(t, fileYAML)); err != nil {
			t.Fatalf("set --config: %v", err)
		}
		t.Setenv("VSPHERE_URL", "https://env-vc.lab/sdk")
		t.Setenv("VSPHERE_TIMEOUT", "45s")
		cfg, err := Load(fs)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.URL != "https://env-vc.lab/sdk" {
			t.Errorf("URL = %q, want env value over file", cfg.URL)
		}
		if cfg.Timeout.String() != "45s" {
			t.Errorf("Timeout = %s, want env value over file", cfg.Timeout)
		}
		if !cfg.Insecure {
			t.Errorf("Insecure = false, want file value (env unset)")
		}
	})

	t.Run("flag overrides env and file", func(t *testing.T) {
		fs := newFlags(t)
		if err := fs.Set("config", writeConfigFile(t, fileYAML)); err != nil {
			t.Fatalf("set --config: %v", err)
		}
		t.Setenv("VSPHERE_URL", "https://env-vc.lab/sdk")
		t.Setenv("VSPHERE_INSECURE", "true")
		for _, set := range []struct{ k, v string }{
			{"url", "https://flag-vc.lab/sdk"},
			{"timeout", "90s"},
			{"insecure", "false"},
			{"username", "flag-user"},
		} {
			if err := fs.Set(set.k, set.v); err != nil {
				t.Fatalf("set --%s: %v", set.k, err)
			}
		}
		cfg, err := Load(fs)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.URL != "https://flag-vc.lab/sdk" {
			t.Errorf("URL = %q, want flag value", cfg.URL)
		}
		if cfg.Timeout != 90*time.Second {
			t.Errorf("Timeout = %s, want flag value", cfg.Timeout)
		}
		if cfg.Insecure {
			t.Errorf("Insecure = true, want flag value false over env/file true")
		}
		if cfg.Username != "flag-user" {
			t.Errorf("Username = %q, want flag value", cfg.Username)
		}
	})
}

func TestLoadValidation(t *testing.T) {
	t.Run("missing url errors with actionable message", func(t *testing.T) {
		_, err := Load(newFlags(t))
		if err == nil {
			t.Fatal("Load should fail without any URL source")
		}
		if !strings.Contains(err.Error(), "--url") {
			t.Errorf("error %q should mention --url", err)
		}
	})

	t.Run("non-positive timeout errors", func(t *testing.T) {
		fs := newFlags(t)
		t.Setenv("VSPHERE_URL", "https://vc.lab/sdk")
		t.Setenv("VSPHERE_TIMEOUT", "-5s")
		if _, err := Load(fs); err == nil {
			t.Fatal("Load should fail for a negative timeout")
		}
	})
}

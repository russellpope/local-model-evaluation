package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestConfigPrecedence(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "vsphere.yaml")
	if err := os.WriteFile(filePath, []byte("url: file-url\nusername: file-user\ninsecure: true\ntimeout: 30s\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Layer 1: defaults only.
	t.Run("default", func(t *testing.T) {
		c, fs := setup(t)
		mustParse(t, fs, []string{"--config", ""})
		mustLoad(t, c, "")
		v, err := c.Resolve()
		if err == nil {
			t.Fatalf("expected missing-URL error, got %+v", v)
		}
	})

	// Layer 2: config file beats defaults.
	t.Run("file over default", func(t *testing.T) {
		c, fs := setup(t)
		mustParse(t, fs, []string{"--config", filePath})
		mustLoad(t, c, filePath)
		got := mustResolve(t, c)
		if got.URL != "file-url" || got.Username != "file-user" || !got.Insecure || got.Timeout != 30*time.Second {
			t.Fatalf("file layer wrong: %+v", got)
		}
	})

	// Layer 3: environment beats the config file.
	t.Run("env over file", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "env-url")
		c, fs := setup(t)
		mustParse(t, fs, []string{"--config", filePath})
		mustLoad(t, c, filePath)
		if got := mustResolve(t, c); got.URL != "env-url" {
			t.Fatalf("URL = %q, want env-url", got.URL)
		}
		// Non-overridden keys still come from the file.
		if got := mustResolve(t, c); got.Username != "file-user" {
			t.Fatalf("Username = %q, want file-user", got.Username)
		}
	})

	// Layer 4: flag beats environment (and file).
	t.Run("flag over env", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "env-url")
		c, fs := setup(t)
		mustParse(t, fs, []string{"--url", "flag-url", "--config", filePath})
		mustLoad(t, c, filePath)
		if got := mustResolve(t, c); got.URL != "flag-url" {
			t.Fatalf("URL = %q, want flag-url", got.URL)
		}
	})

	// Env alone (no file) works too, and an unset flag must not mask it.
	t.Run("env beats default", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "env-url")
		t.Setenv("VSPHERE_TIMEOUT", "45s")
		c, _ := setup(t)
		mustLoad(t, c, "")
		if got := mustResolve(t, c); got.URL != "env-url" || got.Timeout != 45*time.Second {
			t.Fatalf("env layer wrong: %+v", got)
		}
	})
}

func setup(t *testing.T) (*Config, *pflag.FlagSet) {
	t.Helper()
	c := New()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := c.RegisterFlags(fs); err != nil {
		t.Fatal(err)
	}
	return c, fs
}

func mustParse(t *testing.T, fs *pflag.FlagSet, args []string) {
	t.Helper()
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
}

func mustLoad(t *testing.T, c *Config, path string) {
	t.Helper()
	if err := c.LoadConfigFile(path); err != nil {
		t.Fatalf("load %q: %v", path, err)
	}
}

func mustResolve(t *testing.T, c *Config) Values {
	t.Helper()
	v, err := c.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	return v
}

func TestBadConfigFileError(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(bad, []byte("url: [unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := New()
	if err := c.LoadConfigFile(bad); err == nil {
		t.Fatal("expected parse error")
	}
}

package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"vsphere-inventory/internal/config"
)

// clearEnv removes any VSPHERE_ variables the surrounding environment may have
// set so the precedence assertions are deterministic, restoring them after.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"VSPHERE_URL", "VSPHERE_USERNAME", "VSPHERE_PASSWORD",
		"VSPHERE_INSECURE", "VSPHERE_TIMEOUT",
	} {
		if orig, ok := os.LookupEnv(k); ok {
			t.Cleanup(func() { os.Setenv(k, orig) })
		}
		os.Unsetenv(k)
	}
}

func writeConfigFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	contents := "url: file-url\nusername: file-user\ntimeout: 30s\ninsecure: true\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func TestConfigPrecedence(t *testing.T) {
	clearEnv(t)
	cfgPath := writeConfigFile(t)

	t.Run("default wins when nothing else is set", func(t *testing.T) {
		v := config.New()
		cfg := config.Resolve(v)
		if cfg.URL != "" {
			t.Errorf("URL = %q, want empty default", cfg.URL)
		}
		if cfg.Insecure != false {
			t.Errorf("Insecure = %v, want default false", cfg.Insecure)
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("Timeout = %v, want default 60s", cfg.Timeout)
		}
	})

	t.Run("config file overrides default", func(t *testing.T) {
		v := config.New()
		if err := config.LoadConfigFile(v, cfgPath); err != nil {
			t.Fatalf("LoadConfigFile: %v", err)
		}
		cfg := config.Resolve(v)
		if cfg.URL != "file-url" {
			t.Errorf("URL = %q, want file-url", cfg.URL)
		}
		if cfg.Username != "file-user" {
			t.Errorf("Username = %q, want file-user", cfg.Username)
		}
		if cfg.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want 30s from file", cfg.Timeout)
		}
		if cfg.Insecure != true {
			t.Errorf("Insecure = %v, want true from file", cfg.Insecure)
		}
	})

	t.Run("env overrides config file", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "env-url")
		t.Setenv("VSPHERE_TIMEOUT", "20s")
		v := config.New()
		if err := config.LoadConfigFile(v, cfgPath); err != nil {
			t.Fatalf("LoadConfigFile: %v", err)
		}
		cfg := config.Resolve(v)
		if cfg.URL != "env-url" {
			t.Errorf("URL = %q, want env-url (env beats file)", cfg.URL)
		}
		if cfg.Timeout != 20*time.Second {
			t.Errorf("Timeout = %v, want 20s (env beats file)", cfg.Timeout)
		}
		// A key set only in the file still resolves from the file.
		if cfg.Username != "file-user" {
			t.Errorf("Username = %q, want file-user", cfg.Username)
		}
	})

	t.Run("flag overrides env and config file", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "env-url")
		t.Setenv("VSPHERE_TIMEOUT", "20s")

		cmd := &cobra.Command{Use: "test"}
		config.RegisterFlags(cmd)
		if err := cmd.ParseFlags([]string{"--url=flag-url", "--timeout=10s"}); err != nil {
			t.Fatalf("parse flags: %v", err)
		}

		v := config.New()
		if err := config.LoadConfigFile(v, cfgPath); err != nil {
			t.Fatalf("LoadConfigFile: %v", err)
		}
		if err := config.BindFlags(v, cmd); err != nil {
			t.Fatalf("BindFlags: %v", err)
		}
		cfg := config.Resolve(v)
		if cfg.URL != "flag-url" {
			t.Errorf("URL = %q, want flag-url (flag beats env and file)", cfg.URL)
		}
		if cfg.Timeout != 10*time.Second {
			t.Errorf("Timeout = %v, want 10s (flag beats env and file)", cfg.Timeout)
		}
	})
}

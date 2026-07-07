package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadPrecedence(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(cfgFile, []byte(`
url: https://file.example.com/sdk
username: fileuser
password: filepass
insecure: false
timeout: 30s
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test 1: Config file values are used when no flags or env vars set
	t.Run("config file only", func(t *testing.T) {
		os.Unsetenv("VSPHERE_URL")
		os.Unsetenv("VSPHERE_USERNAME")
		os.Unsetenv("VSPHERE_PASSWORD")
		os.Unsetenv("VSPHERE_INSECURE")
		os.Unsetenv("VSPHERE_TIMEOUT")

		cfg, err := Load(map[string]string{"config": cfgFile}, "VSPHERE")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://file.example.com/sdk" {
			t.Errorf("URL = %q, want file URL", cfg.URL)
		}
		if cfg.Username != "fileuser" {
			t.Errorf("Username = %q, want fileuser", cfg.Username)
		}
		if cfg.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
		}
	})

	// Test 2: Env var overrides config file
	t.Run("env overrides file", func(t *testing.T) {
		os.Setenv("VSPHERE_URL", "https://env.example.com/sdk")
		os.Setenv("VSPHERE_USERNAME", "envuser")
		defer func() {
			os.Unsetenv("VSPHERE_URL")
			os.Unsetenv("VSPHERE_USERNAME")
		}()

		cfg, err := Load(map[string]string{"config": cfgFile}, "VSPHERE")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://env.example.com/sdk" {
			t.Errorf("URL = %q, want env URL", cfg.URL)
		}
		if cfg.Username != "envuser" {
			t.Errorf("Username = %q, want envuser", cfg.Username)
		}
	})

	// Test 3: Flag overrides env var
	t.Run("flag overrides env", func(t *testing.T) {
		os.Setenv("VSPHERE_URL", "https://env.example.com/sdk")
		defer os.Unsetenv("VSPHERE_URL")

		cfg, err := Load(map[string]string{
			"config": cfgFile,
			"url":    "https://flag.example.com/sdk",
		}, "VSPHERE")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://flag.example.com/sdk" {
			t.Errorf("URL = %q, want flag URL", cfg.URL)
		}
	})

	// Test 4: Default when nothing set
	t.Run("defaults", func(t *testing.T) {
		os.Unsetenv("VSPHERE_URL")
		os.Unsetenv("VSPHERE_USERNAME")
		os.Unsetenv("VSPHERE_PASSWORD")
		os.Unsetenv("VSPHERE_INSECURE")
		os.Unsetenv("VSPHERE_TIMEOUT")

		cfg, err := Load(map[string]string{}, "VSPHERE")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.URL != "https://localhost/sdk" {
			t.Errorf("URL = %q, want default", cfg.URL)
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
		}
		if cfg.Insecure != false {
			t.Errorf("Insecure = %v, want false", cfg.Insecure)
		}
	})
}

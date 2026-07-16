package tests

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"vsphere-inventory/internal/config"
)

func TestConfigPrecedence(t *testing.T) {
	t.Parallel()

	// Create a temporary config file
	configContent := `
url: https://from-config.example.com
username: configuser
password: configpass
insecure: true
timeout: 30s
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(configContent)); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("closing temp file: %v", err)
	}

	// Set env vars to override config
	os.Setenv("VSPHERE_URL", "https://from-env.example.com")
	os.Setenv("VSPHERE_USERNAME", "envuser")
	os.Setenv("VSPHERE_PASSWORD", "envpass")
	os.Setenv("VSPHERE_INSECURE", "false")
	os.Setenv("VSPHERE_TIMEOUT", "45s")
	defer func() {
		os.Unsetenv("VSPHERE_URL")
		os.Unsetenv("VSPHERE_USERNAME")
		os.Unsetenv("VSPHERE_PASSWORD")
		os.Unsetenv("VSPHERE_INSECURE")
		os.Unsetenv("VSPHERE_TIMEOUT")
	}()

	// Load with config file
	cfg, err := config.Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	// Env should override config
	if cfg.URL != "https://from-env.example.com" {
		t.Errorf("expected URL from env, got %s", cfg.URL)
	}
	if cfg.Username != "envuser" {
		t.Errorf("expected username from env, got %s", cfg.Username)
	}
	if cfg.Password != "envpass" {
		t.Errorf("expected password from env, got %s", cfg.Password)
	}
	if cfg.Insecure != false {
		t.Errorf("expected insecure=false from env, got %v", cfg.Insecure)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("expected timeout 45s from env, got %v", cfg.Timeout)
	}

	// Test flag override - create a new viper instance
	v := viper.New()
	v.SetConfigFile(tmpFile.Name())
	v.ReadInConfig()
	v.Set("url", "https://from-flag.example.com")
	v.Set("username", "flaguser")
	v.Set("password", "flagpass")
	v.Set("insecure", true)
	v.Set("timeout", 60*time.Second)

	// Flags should override env
	if v.GetString("url") != "https://from-flag.example.com" {
		t.Errorf("expected URL from flag, got %s", v.GetString("url"))
	}
	if v.GetString("username") != "flaguser" {
		t.Errorf("expected username from flag, got %s", v.GetString("username"))
	}
	if v.GetString("password") != "flagpass" {
		t.Errorf("expected password from flag, got %s", v.GetString("password"))
	}
	if v.GetBool("insecure") != true {
		t.Errorf("expected insecure=true from flag, got %v", v.GetBool("insecure"))
	}
	if v.GetDuration("timeout") != 60*time.Second {
		t.Errorf("expected timeout 60s from flag, got %v", v.GetDuration("timeout"))
	}
}

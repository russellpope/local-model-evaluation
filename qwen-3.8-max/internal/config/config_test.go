package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const configYAML = `url: https://file.example.lab/sdk
username: file-user
password: file-pass
insecure: true
timeout: 5s
`

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	return path
}

func newFlagSet() *pflag.FlagSet {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("url", "", "")
	flags.String("username", "", "")
	flags.String("password", "", "")
	flags.Bool("insecure", false, "")
	flags.String("timeout", "", "")
	return flags
}

func newViperWithFile(t *testing.T, flags *pflag.FlagSet) *viper.Viper {
	t.Helper()
	v := NewViper()
	v.SetConfigFile(writeConfigFile(t, configYAML))
	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := BindFlags(v, flags); err != nil {
		t.Fatalf("bind flags: %v", err)
	}
	return v
}

func TestConfigFileValues(t *testing.T) {
	v := newViperWithFile(t, newFlagSet())
	cfg, err := Resolve(v)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.URL != "https://file.example.lab/sdk" {
		t.Errorf("URL = %q, want file value", cfg.URL)
	}
	if cfg.Username != "file-user" || cfg.Password != "file-pass" {
		t.Errorf("credentials = %q/%q, want file values", cfg.Username, cfg.Password)
	}
	if !cfg.Insecure {
		t.Error("Insecure = false, want true from file")
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", cfg.Timeout)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	t.Setenv("VSPHERE_URL", "https://env.example.lab/sdk")
	t.Setenv("VSPHERE_USERNAME", "env-user")
	t.Setenv("VSPHERE_TIMEOUT", "90s")

	v := newViperWithFile(t, newFlagSet())
	cfg, err := Resolve(v)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.URL != "https://env.example.lab/sdk" {
		t.Errorf("URL = %q, want env value to override file", cfg.URL)
	}
	if cfg.Username != "env-user" {
		t.Errorf("Username = %q, want env value to override file", cfg.Username)
	}
	if cfg.Password != "file-pass" {
		t.Errorf("Password = %q, want file value when env is unset", cfg.Password)
	}
	if cfg.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want env value 90s", cfg.Timeout)
	}
}

func TestFlagOverridesEnvAndFile(t *testing.T) {
	t.Setenv("VSPHERE_URL", "https://env.example.lab/sdk")

	flags := newFlagSet()
	if err := flags.Set("url", "https://flag.example.lab/sdk"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	if err := flags.Set("insecure", "false"); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	v := newViperWithFile(t, flags)
	cfg, err := Resolve(v)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.URL != "https://flag.example.lab/sdk" {
		t.Errorf("URL = %q, want flag value to override env and file", cfg.URL)
	}
	if cfg.Insecure {
		t.Error("Insecure = true, want flag (false) to override file (true)")
	}
	if cfg.Username != "file-user" {
		t.Errorf("Username = %q, want file value when flag and env are unset", cfg.Username)
	}
}

func TestDefaults(t *testing.T) {
	v := NewViper()
	if err := BindFlags(v, newFlagSet()); err != nil {
		t.Fatalf("bind flags: %v", err)
	}
	cfg, err := Resolve(v)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.URL != "" || cfg.Username != "" || cfg.Password != "" {
		t.Errorf("expected empty defaults, got %q %q %q", cfg.URL, cfg.Username, cfg.Password)
	}
	if cfg.Insecure {
		t.Error("Insecure default = true, want false")
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout default = %v, want 60s", cfg.Timeout)
	}
}

func TestInvalidTimeout(t *testing.T) {
	t.Setenv("VSPHERE_TIMEOUT", "banana")
	v := NewViper()
	if _, err := Resolve(v); err == nil {
		t.Fatal("Resolve with invalid timeout: expected error, got nil")
	}
}

func TestNonPositiveTimeout(t *testing.T) {
	t.Setenv("VSPHERE_TIMEOUT", "0s")
	v := NewViper()
	if _, err := Resolve(v); err == nil {
		t.Fatal("Resolve with zero timeout: expected error, got nil")
	}
}

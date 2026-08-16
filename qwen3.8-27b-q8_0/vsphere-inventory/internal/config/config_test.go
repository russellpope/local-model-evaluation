package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// TestLoadPrecedence verifies that a flag beats an env var, which beats the
// config file, which beats the built-in default.
func TestLoadPrecedence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	content := "url: file-url\nusername: file-user\npassword: file-pass\ninsecure: true\ntimeout: 30s\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	v, err := NewViper(file)
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("url", "", "")
	fs.String("username", "", "")
	fs.String("password", "", "")
	fs.Bool("insecure", false, "")
	fs.Duration("timeout", 60*time.Second, "")
	if err := BindFlags(v, fs); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}

	// Config file beats the built-in defaults.
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "file-url" {
		t.Errorf("url = %q, want file-url", cfg.URL)
	}
	if cfg.Username != "file-user" {
		t.Errorf("username = %q, want file-user", cfg.Username)
	}
	if cfg.Password != "file-pass" {
		t.Errorf("password = %q, want file-pass", cfg.Password)
	}
	if !cfg.Insecure {
		t.Error("insecure = false, want true from config file")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("timeout = %s, want 30s from config file", cfg.Timeout)
	}

	// Env vars beat the config file.
	t.Setenv("VSPHERE_URL", "env-url")
	t.Setenv("VSPHERE_USERNAME", "env-user")
	t.Setenv("VSPHERE_INSECURE", "false")

	cfg, err = Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "env-url" {
		t.Errorf("url = %q, want env-url", cfg.URL)
	}
	if cfg.Username != "env-user" {
		t.Errorf("username = %q, want env-user", cfg.Username)
	}
	if cfg.Password != "file-pass" {
		t.Errorf("password = %q, want file-pass (no env override)", cfg.Password)
	}
	if cfg.Insecure {
		t.Error("insecure = true, want false from env")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("timeout = %s, want 30s from config file", cfg.Timeout)
	}

	// A changed flag beats the env var.
	if err := fs.Set("url", "flag-url"); err != nil {
		t.Fatalf("fs.Set: %v", err)
	}
	if err := fs.Set("timeout", "45s"); err != nil {
		t.Fatalf("fs.Set: %v", err)
	}

	cfg, err = Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "flag-url" {
		t.Errorf("url = %q, want flag-url", cfg.URL)
	}
	if cfg.Username != "env-user" {
		t.Errorf("username = %q, want env-user (flag not set)", cfg.Username)
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("timeout = %s, want 45s from flag", cfg.Timeout)
	}
}

// TestLoadDefaults verifies the built-in defaults when nothing else is set.
func TestLoadDefaults(t *testing.T) {
	v, err := NewViper("")
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("url", "", "")
	fs.String("username", "", "")
	fs.String("password", "", "")
	fs.Bool("insecure", false, "")
	fs.Duration("timeout", 60*time.Second, "")
	if err := BindFlags(v, fs); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "" {
		t.Errorf("url = %q, want empty default", cfg.URL)
	}
	if cfg.Insecure {
		t.Error("insecure = true, want false default")
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("timeout = %s, want 60s default", cfg.Timeout)
	}
}

// TestLoadInvalidTimeout verifies that a malformed timeout is rejected.
func TestLoadInvalidTimeout(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("timeout: not-a-duration\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	v, err := NewViper(file)
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("url", "", "")
	fs.String("username", "", "")
	fs.String("password", "", "")
	fs.Bool("insecure", false, "")
	fs.Duration("timeout", 60*time.Second, "")
	if err := BindFlags(v, fs); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}

	if _, err := Load(v); err == nil {
		t.Fatal("Load: expected an error for an invalid timeout, got nil")
	}
}

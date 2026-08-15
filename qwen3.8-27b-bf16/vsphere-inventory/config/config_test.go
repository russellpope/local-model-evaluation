package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func newFlagSet(t *testing.T) *pflag.FlagSet {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("url", "", "")
	fs.String("username", "", "")
	fs.String("password", "", "")
	fs.Bool("insecure", false, "")
	fs.Duration("timeout", 0, "")
	return fs
}

func TestPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	const fileContent = "url: file-url\nusername: file-user\ntimeout: 5s\n"
	if err := os.WriteFile(path, []byte(fileContent), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("VSPHERE_URL", "env-url")
	t.Setenv("VSPHERE_USERNAME", "env-user")

	v, err := NewViper(path)
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}

	// Layering so far: defaults < file < env.
	if err := BindFlags(v, newFlagSet(t)); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "env-url" {
		t.Errorf("URL = %q, want env-url (env must override file)", cfg.URL)
	}
	if cfg.Username != "env-user" {
		t.Errorf("Username = %q, want env-user (env must override file)", cfg.Username)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s (from config file)", cfg.Timeout)
	}
	if cfg.Insecure != DefaultInsecure {
		t.Errorf("Insecure = %v, want %v (built-in default)", cfg.Insecure, DefaultInsecure)
	}

	// Now a changed flag must override both env and file.
	fs := newFlagSet(t)
	if err := fs.Set("url", "flag-url"); err != nil {
		t.Fatalf("set url flag: %v", err)
	}
	if err := BindFlags(v, fs); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}
	cfg, err = Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "flag-url" {
		t.Errorf("URL = %q, want flag-url (flag must override env and file)", cfg.URL)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s (flag for timeout was not set, file value must stand)", cfg.Timeout)
	}
}

func TestDefaults(t *testing.T) {
	v, err := NewViper("")
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}
	if err := BindFlags(v, newFlagSet(t)); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}

	t.Setenv("VSPHERE_URL", "https://vc.lab/sdk")
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want default %v", cfg.Timeout, DefaultTimeout)
	}
	if cfg.Insecure != DefaultInsecure {
		t.Errorf("Insecure = %v, want default %v", cfg.Insecure, DefaultInsecure)
	}
}

func TestLoadRequiresURL(t *testing.T) {
	v, err := NewViper("")
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}
	if err := BindFlags(v, newFlagSet(t)); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}
	if _, err := Load(v); err == nil {
		t.Fatalf("Load: expected an error when no URL is configured anywhere")
	}
}

func TestLoadRejectsBadTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("url: https://vc.lab/sdk\ntimeout: soon\n"), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}
	v, err := NewViper(path)
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}
	if err := BindFlags(v, newFlagSet(t)); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}
	if _, err := Load(v); err == nil {
		t.Fatalf("Load: expected an error for unparseable timeout")
	}
}

func TestLoadEnvOnly(t *testing.T) {
	t.Setenv("VSPHERE_URL", "https://env.lab/sdk")
	t.Setenv("VSPHERE_USERNAME", "env-user")
	t.Setenv("VSPHERE_PASSWORD", "env-pass")
	t.Setenv("VSPHERE_INSECURE", "true")
	t.Setenv("VSPHERE_TIMEOUT", "42s")

	v, err := NewViper("")
	if err != nil {
		t.Fatalf("NewViper: %v", err)
	}
	if err := BindFlags(v, newFlagSet(t)); err != nil {
		t.Fatalf("BindFlags: %v", err)
	}
	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "https://env.lab/sdk" {
		t.Errorf("URL = %q, want https://env.lab/sdk", cfg.URL)
	}
	if cfg.Username != "env-user" {
		t.Errorf("Username = %q, want env-user", cfg.Username)
	}
	if cfg.Password != "env-pass" {
		t.Errorf("Password = %q, want env-pass", cfg.Password)
	}
	if !cfg.Insecure {
		t.Errorf("Insecure = false, want true from VSPHERE_INSECURE")
	}
	if cfg.Timeout != 42*time.Second {
		t.Errorf("Timeout = %v, want 42s", cfg.Timeout)
	}
}

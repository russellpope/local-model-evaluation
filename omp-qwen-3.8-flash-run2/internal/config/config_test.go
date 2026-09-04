package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// newFlags builds a fresh conn flag set (same shape the CLI registers).
func newFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("run", pflag.ContinueOnError)
	fs.String(KeyURL, "", "")
	fs.String(KeyUsername, "", "")
	fs.String(KeyPassword, "", "")
	fs.Bool(KeyInsecure, false, "")
	fs.String(KeyTimeout, "60s", "")
	fs.String(KeyConfig, "", "")
	return fs
}

// resolve parses argv into flags, wires viper, reads the config file if
// given, and returns the resolved settings.
func resolve(t *testing.T, argv []string) Settings {
	t.Helper()
	fs := newFlags()
	if err := fs.Parse(argv); err != nil {
		t.Fatal(err)
	}
	v, err := NewViper(fs)
	if err != nil {
		t.Fatal(err)
	}
	if err := ReadFile(v, v.GetString(KeyConfig)); err != nil {
		t.Fatal(err)
	}
	s, err := Load(v)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPrecedence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte(
		"url: https://file.example/sdk\nusername: file-user\npassword: file-pass\ntimeout: 30s\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("defaults", func(t *testing.T) {
		s := resolve(t, nil)
		if s.Timeout != 60*time.Second {
			t.Errorf("default timeout = %v, want 60s", s.Timeout)
		}
		if s.URL != "" || s.Insecure {
			t.Errorf("unexpected defaults: %+v", s)
		}
	})

	t.Run("file beats default", func(t *testing.T) {
		s := resolve(t, []string{"--config", file})
		if s.URL != "https://file.example/sdk" {
			t.Errorf("file url = %q", s.URL)
		}
		if s.Timeout != 30*time.Second {
			t.Errorf("file timeout = %v, want 30s (file over default)", s.Timeout)
		}
	})

	t.Run("env beats file", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://env.example/sdk")
		t.Setenv("VSPHERE_TIMEOUT", "45s")
		s := resolve(t, []string{"--config", file})
		if s.URL != "https://env.example/sdk" {
			t.Errorf("env url = %q, want env to override file", s.URL)
		}
		if s.Timeout != 45*time.Second {
			t.Errorf("env timeout = %v, want 45s (env over file)", s.Timeout)
		}
		// Keys set only in the file survive untouched.
		if s.Username != "file-user" {
			t.Errorf("username = %q, want file value preserved", s.Username)
		}
	})

	t.Run("flag beats env", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://env.example/sdk")
		t.Setenv("VSPHERE_PASSWORD", "env-pass")
		t.Setenv("VSPHERE_TIMEOUT", "45s")
		s := resolve(t, []string{
			"--config", file,
			"--url", "https://flag.example/sdk",
			"--password", "flag-pass",
			"--timeout", "10s",
			"--insecure=true",
		})
		if s.URL != "https://flag.example/sdk" {
			t.Errorf("flag url = %q, want flag to override env", s.URL)
		}
		if s.Password != "flag-pass" {
			t.Errorf("flag password = %q, want flag to override env", s.Password)
		}
		if s.Timeout != 10*time.Second {
			t.Errorf("flag timeout = %v, want 10s", s.Timeout)
		}
		if !s.Insecure {
			t.Error("flag insecure not honored")
		}
		// Unset flag must not shadow the file.
		if s.Username != "file-user" {
			t.Errorf("username = %q, want file value (flag unset, env unset)", s.Username)
		}
	})

	t.Run("unset flag yields to env", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://env.example/sdk")
		s := resolve(t, nil)
		if s.URL != "https://env.example/sdk" {
			t.Errorf("url = %q; an unset flag must not win over env", s.URL)
		}
	})

	t.Run("full chain flag > env > file > default", func(t *testing.T) {
		// timeout: flag 10s must win over env 45s, file 30s, default 60s.
		// url/password: file value survives (env/flag unset for those keys).
		t.Setenv("VSPHERE_TIMEOUT", "45s")
		s := resolve(t, []string{"--config", file, "--timeout", "10s"})
		if s.Timeout != 10*time.Second {
			t.Errorf("timeout = %v, want flag(10s)", s.Timeout)
		}
		if s.URL != "https://file.example/sdk" {
			t.Errorf("url = %q, want file value", s.URL)
		}
		if s.Password != "file-pass" {
			t.Errorf("password = %q, want file value", s.Password)
		}
	})

	t.Run("bad config file error is wrapped", func(t *testing.T) {
		missing := filepath.Join(dir, "nope.yaml")
		fs := newFlags()
		v, err := NewViper(fs)
		if err != nil {
			t.Fatal(err)
		}
		err = ReadFile(v, missing)
		if err == nil {
			t.Fatal("want error for missing config file")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("error should wrap the underlying file error: %v", err)
		}
	})

	t.Run("non-positive timeout rejected", func(t *testing.T) {
		fs := newFlags()
		if err := fs.Parse([]string{"--timeout", "0s"}); err != nil {
			t.Fatal(err)
		}
		v, err := NewViper(fs)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Load(v); err == nil {
			t.Fatal("want error for timeout=0")
		}
	})
}

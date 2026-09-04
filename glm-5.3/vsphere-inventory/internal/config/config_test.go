package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

// newBoundViper builds a viper instance with defaults, an optional config
// file and environment binding (the flag tier is covered by the cmd
// package's precedence test, which uses the production wiring).
func newBoundViper(t *testing.T, configYAML string) *viper.Viper {
	t.Helper()

	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	for key, def := range Defaults() {
		v.SetDefault(key, def)
	}
	if configYAML != "" {
		dir := t.TempDir()
		path := dir + "/config.yaml"
		if err := writeFile(path, configYAML); err != nil {
			t.Fatal(err)
		}
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
	}
	return v
}

func TestLoad(t *testing.T) {
	t.Run("env beats config file and default", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://env-vc.lab/sdk")
		t.Setenv("VSPHERE_USERNAME", "env-user")
		t.Setenv("VSPHERE_INSECURE", "true")
		t.Setenv("VSPHERE_TIMEOUT", "30s")

		cfg, err := Load(newBoundViper(t, `
url: https://file-vc.lab/sdk
username: file-user
password: file-pass
insecure: false
timeout: 45s
`))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.URL != "https://env-vc.lab/sdk" {
			t.Errorf("url = %q, want env value", cfg.URL)
		}
		if cfg.Username != "env-user" {
			t.Errorf("username = %q, want env value", cfg.Username)
		}
		if !cfg.Insecure {
			t.Error("insecure = false, want env value true")
		}
		if cfg.Timeout != 30*time.Second {
			t.Errorf("timeout = %s, want 30s", cfg.Timeout)
		}
	})

	t.Run("config file beats default", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "")
		t.Setenv("VSPHERE_USERNAME", "")
		t.Setenv("VSPHERE_TIMEOUT", "")
		t.Setenv("VSPHERE_INSECURE", "")

		cfg, err := Load(newBoundViper(t, `
url: https://file-vc.lab/sdk
username: file-user
timeout: 45s
insecure: true
`))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.URL != "https://file-vc.lab/sdk" || cfg.Username != "file-user" {
			t.Errorf("got %+v, want config file values", cfg)
		}
		if cfg.Timeout != 45*time.Second {
			t.Errorf("timeout = %s, want 45s", cfg.Timeout)
		}
		if !cfg.Insecure {
			t.Error("insecure = false, want true from config file")
		}
	})

	t.Run("defaults", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://vc.lab/sdk")
		t.Setenv("VSPHERE_USERNAME", "u")

		cfg, err := Load(newBoundViper(t, ""))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Insecure {
			t.Error("insecure default = true, want false")
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("timeout default = %s, want 60s", cfg.Timeout)
		}
	})
}

func TestLoadErrors(t *testing.T) {
	t.Run("missing url", func(t *testing.T) {
		if _, err := Load(newBoundViper(t, "")); err == nil {
			t.Fatal("expected error for missing url")
		}
	})
	t.Run("missing username", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://vc.lab/sdk")
		if _, err := Load(newBoundViper(t, "")); err == nil {
			t.Fatal("expected error for missing username")
		}
	})
	t.Run("invalid timeout", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://vc.lab/sdk")
		t.Setenv("VSPHERE_USERNAME", "u")
		t.Setenv("VSPHERE_TIMEOUT", "not-a-duration")
		if _, err := Load(newBoundViper(t, "")); err == nil {
			t.Fatal("expected error for invalid timeout")
		}
	})
	t.Run("negative timeout", func(t *testing.T) {
		t.Setenv("VSPHERE_URL", "https://vc.lab/sdk")
		t.Setenv("VSPHERE_USERNAME", "u")
		t.Setenv("VSPHERE_TIMEOUT", "-5s")
		if _, err := Load(newBoundViper(t, "")); err == nil {
			t.Fatal("expected error for negative timeout")
		}
	})
}

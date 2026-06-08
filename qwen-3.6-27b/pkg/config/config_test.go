package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestConfigPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")

	yamlContent := `url: https://file-config.example/sdk
username: fileuser
password: filepass
insecure: true
timeout: 30s
`
	if err := os.WriteFile(configFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Run("default values", func(t *testing.T) {
		v := viper.New()
		v.SetDefault("url", "")
		v.SetDefault("username", "")
		v.SetDefault("password", "")
		v.SetDefault("insecure", false)
		v.SetDefault("timeout", "60s")
		v.SetEnvPrefix(EnvPrefix)
		v.AutomaticEnv()

		cleanEnv(t)
		cfg, err := Load(v)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.Insecure != false {
			t.Errorf("default insecure = %v, want false", cfg.Insecure)
		}
		if cfg.Timeout != 60*time.Second {
			t.Errorf("default timeout = %v, want 60s", cfg.Timeout)
		}
	})

	t.Run("config file values", func(t *testing.T) {
		v := viper.New()
		v.SetDefault("url", "")
		v.SetDefault("username", "")
		v.SetDefault("password", "")
		v.SetDefault("insecure", false)
		v.SetDefault("timeout", "60s")
		v.SetEnvPrefix(EnvPrefix)
		v.AutomaticEnv()
		v.Set("config", configFile)

		cleanEnv(t)
		cfg, err := Load(v)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.URL != "https://file-config.example/sdk" {
			t.Errorf("URL = %q, want %q", cfg.URL, "https://file-config.example/sdk")
		}
		if cfg.Username != "fileuser" {
			t.Errorf("Username = %q, want %q", cfg.Username, "fileuser")
		}
		if cfg.Password != "filepass" {
			t.Errorf("Password = %q, want %q", cfg.Password, "filepass")
		}
		if cfg.Insecure != true {
			t.Errorf("Insecure = %v, want true", cfg.Insecure)
		}
		if cfg.Timeout != 30*time.Second {
			t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
		}
	})

	t.Run("env vars override config file", func(t *testing.T) {
		v := viper.New()
		v.SetDefault("url", "")
		v.SetDefault("username", "")
		v.SetDefault("password", "")
		v.SetDefault("insecure", false)
		v.SetDefault("timeout", "60s")
		v.SetEnvPrefix(EnvPrefix)
		v.AutomaticEnv()
		v.Set("config", configFile)

		os.Setenv("VSPHERE_URL", "https://env-config.example/sdk")
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

		cfg, err := Load(v)
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}
		if cfg.URL != "https://env-config.example/sdk" {
			t.Errorf("URL = %q, want %q", cfg.URL, "https://env-config.example/sdk")
		}
		if cfg.Username != "envuser" {
			t.Errorf("Username = %q, want %q", cfg.Username, "envuser")
		}
		if cfg.Password != "envpass" {
			t.Errorf("Password = %q, want %q", cfg.Password, "envpass")
		}
		if cfg.Insecure != false {
			t.Errorf("Insecure = %v, want false", cfg.Insecure)
		}
		if cfg.Timeout != 45*time.Second {
			t.Errorf("Timeout = %v, want 45s", cfg.Timeout)
		}
	})
}

func TestValidate(t *testing.T) {
	t.Run("missing URL", func(t *testing.T) {
		cfg := &Config{Username: "user", Password: "pass"}
		if err := Validate(cfg); err == nil {
			t.Error("expected error for missing URL")
		}
	})

	t.Run("missing username", func(t *testing.T) {
		cfg := &Config{URL: "https://vc/sdk", Password: "pass"}
		if err := Validate(cfg); err == nil {
			t.Error("expected error for missing username")
		}
	})

	t.Run("missing password", func(t *testing.T) {
		cfg := &Config{URL: "https://vc/sdk", Username: "user"}
		if err := Validate(cfg); err == nil {
			t.Error("expected error for missing password")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{URL: "https://vc/sdk", Username: "user", Password: "pass"}
		if err := Validate(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func cleanEnv(t *testing.T) {
	t.Helper()
	prefix := "VSPHERE_"
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, prefix) {
			key := strings.SplitN(e, "=", 2)[0]
			os.Unsetenv(key)
		}
	}
}

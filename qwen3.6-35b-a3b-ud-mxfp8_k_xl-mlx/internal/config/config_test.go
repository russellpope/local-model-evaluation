package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"vsphere-inventory/internal/config"
)

func TestConfigDefaults(t *testing.T) {
	v := viper.New()
	fs := pflag.NewFlagSet("defaults", pflag.ContinueOnError)

	v.BindPFlags(fs)
	v.AutomaticEnv()
	v.SetEnvPrefix("VSPHERE")

	c, err := config.Load(fs)
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}

	if c.Timeout != 60*1000000000 {
		t.Errorf("Timeout = %v, want 60s", c.Timeout)
	}

	if c.Insecure != false {
		t.Errorf("Insecure = %v, want false", c.Insecure)
	}

	if c.URL != "" {
		t.Errorf("URL = %q, want empty string for default", c.URL)
	}
}

func TestConfigEnvOverride(t *testing.T) {
	os.Setenv("VSPHERE_URL", "https://from-env/sdk")
	defer os.Unsetenv("VSPHERE_URL")

	v := viper.New()
	fs := pflag.NewFlagSet("env-override", pflag.ContinueOnError)

	v.BindPFlags(fs)
	v.AutomaticEnv()
	v.SetEnvPrefix("VSPHERE")

	c, err := config.Load(fs)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if c.URL != "https://from-env/sdk" {
		t.Errorf("URL = %q, want https://from-env/sdk", c.URL)
	}
}

func TestConfigEnvPassword(t *testing.T) {
	os.Setenv("VSPHERE_PASSWORD", "secret-pass")
	defer os.Unsetenv("VSPHERE_PASSWORD")

	v := viper.New()
	fs := pflag.NewFlagSet("env-password", pflag.ContinueOnError)

	v.BindPFlags(fs)
	v.AutomaticEnv()
	v.SetEnvPrefix("VSPHERE")

	c, err := config.Load(fs)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if c.Password != "secret-pass" {
		t.Errorf("Password = %q, want secret-pass", c.Password)
	}
}

func TestConfigPrecedence(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("url: https://from-file/sdk\nusername: fileuser\npassword: filepass\ninsecure: true\ntimeout: 30s\n"), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	// Set env var
	os.Setenv("VSPHERE_URL", "https://from-env/sdk")
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

	fs := pflag.NewFlagSet("precedence", pflag.ContinueOnError)

	c, err := config.Load(fs)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Without flag overrides, env should win over file
	if c.URL != "https://from-env/sdk" {
		t.Errorf("URL = %q, want https://from-env/sdk (env > file)", c.URL)
	}
	if c.Username != "envuser" {
		t.Errorf("Username = %q, want envuser (env > file)", c.Username)
	}
	if c.Password != "envpass" {
		t.Errorf("Password = %q, want envpass (env > file)", c.Password)
	}
	if c.Insecure != false {
		t.Errorf("Insecure = %v, want false (env > file)", c.Insecure)
	}
	if c.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want 45s (env > file)", c.Timeout)
	}

	// Now set flags — flags should win over env
	fs2 := pflag.NewFlagSet("precedence-flags", pflag.ContinueOnError)
	_ = config.BindFlags(fs2)
	fs2.Set("url", "https://from-flag/sdk")
	fs2.Set("username", "flaguser")
	fs2.Set("password", "flagpass")
	fs2.Set("insecure", "true")
	fs2.Set("timeout", "90s")
	fs2.Set("config", cfgFile)

	c2, err := config.Load(fs2)
	if err != nil {
		t.Fatalf("load config with flags: %v", err)
	}

	if c2.URL != "https://from-flag/sdk" {
		t.Errorf("URL = %q, want https://from-flag/sdk (flag > env > file)", c2.URL)
	}
	if c2.Username != "flaguser" {
		t.Errorf("Username = %q, want flaguser (flag > env > file)", c2.Username)
	}
	if c2.Password != "flagpass" {
		t.Errorf("Password = %q, want flagpass (flag > env > file)", c2.Password)
	}
	if c2.Insecure != true {
		t.Errorf("Insecure = %v, want true (flag > env > file)", c2.Insecure)
	}
	if c2.Timeout != 90*time.Second {
		t.Errorf("Timeout = %v, want 90s (flag > env > file)", c2.Timeout)
	}
}

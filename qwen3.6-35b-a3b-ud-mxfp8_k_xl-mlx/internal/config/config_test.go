package config_test

import (
	"os"
	"testing"

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

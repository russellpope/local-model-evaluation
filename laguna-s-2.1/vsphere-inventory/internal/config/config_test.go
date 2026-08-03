package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  Config{URL: "https://vc.lab/sdk", Username: "admin", Password: "pass"},
			wantErr: false,
		},
		{
			name:    "missing url",
			config:  Config{Username: "admin", Password: "pass"},
			wantErr: true,
		},
		{
			name:    "missing username",
			config:  Config{URL: "https://vc.lab/sdk", Password: "pass"},
			wantErr: true,
		},
		{
			name:    "missing password",
			config:  Config{URL: "https://vc.lab/sdk", Username: "admin"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigLoadTimeout(t *testing.T) {
	viper.Reset()
	viper.Set("timeout", "30s")

	c := New()
	err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", c.Timeout)
	}
}

func TestConfigLoadInvalidTimeout(t *testing.T) {
	viper.Reset()
	viper.Set("timeout", "invalid")

	c := New()
	err := c.Load()
	if err == nil {
		t.Error("Load() should return error for invalid timeout")
	}
}

func TestConfigLoadFromEnv(t *testing.T) {
	viper.Reset()
	viper.SetEnvPrefix("VSPHERE")
	viper.AutomaticEnv()

	os.Setenv("VSPHERE_URL", "https://env.lab/sdk")
	os.Setenv("VSPHERE_USERNAME", "envuser")
	os.Setenv("VSPHERE_PASSWORD", "envpass")
	os.Setenv("VSPHERE_INSECURE", "true")
	os.Setenv("VSPHERE_TIMEOUT", "45s")
	defer os.Unsetenv("VSPHERE_URL")
	defer os.Unsetenv("VSPHERE_USERNAME")
	defer os.Unsetenv("VSPHERE_PASSWORD")
	defer os.Unsetenv("VSPHERE_INSECURE")
	defer os.Unsetenv("VSPHERE_TIMEOUT")

	c := New()
	err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.URL != "https://env.lab/sdk" {
		t.Errorf("URL = %q, want %q", c.URL, "https://env.lab/sdk")
	}
	if c.Username != "envuser" {
		t.Errorf("Username = %q, want %q", c.Username, "envuser")
	}
	if c.Password != "envpass" {
		t.Errorf("Password = %q, want %q", c.Password, "envpass")
	}
	if !c.Insecure {
		t.Error("Insecure should be true")
	}
	if c.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want 45s", c.Timeout)
	}
}

func TestConfigPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `url: https://file.lab/sdk
username: fileuser
password: filepass
insecure: false
timeout: 10s
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	viper.Reset()
	viper.SetEnvPrefix("VSPHERE")
	viper.AutomaticEnv()

	os.Setenv("VSPHERE_URL", "https://env.lab/sdk")
	os.Setenv("VSPHERE_USERNAME", "envuser")
	os.Setenv("VSPHERE_PASSWORD", "envpass")
	os.Setenv("VSPHERE_INSECURE", "true")
	os.Setenv("VSPHERE_TIMEOUT", "20s")
	defer os.Unsetenv("VSPHERE_URL")
	defer os.Unsetenv("VSPHERE_USERNAME")
	defer os.Unsetenv("VSPHERE_PASSWORD")
	defer os.Unsetenv("VSPHERE_INSECURE")
	defer os.Unsetenv("VSPHERE_TIMEOUT")

	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("reading config file: %v", err)
	}

	c := New()
	err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.URL != "https://env.lab/sdk" {
		t.Errorf("URL = %q, want %q (env should override file)", c.URL, "https://env.lab/sdk")
	}
	if c.Username != "envuser" {
		t.Errorf("Username = %q, want %q (env should override file)", c.Username, "envuser")
	}
	if c.Password != "envpass" {
		t.Errorf("Password = %q, want %q (env should override file)", c.Password, "envpass")
	}
	if !c.Insecure {
		t.Error("Insecure should be true (env should override file)")
	}
	if c.Timeout != 20*time.Second {
		t.Errorf("Timeout = %v, want 20s (env should override file)", c.Timeout)
	}
}

func TestConfigPrecedenceFileOverDefault(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	configContent := `url: https://file.lab/sdk
username: fileuser
password: filepass
insecure: false
timeout: 10s
`
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	viper.Reset()
	viper.SetEnvPrefix("VSPHERE")
	viper.AutomaticEnv()
	viper.SetDefault("timeout", "60s")
	viper.SetDefault("insecure", false)

	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("reading config file: %v", err)
	}

	c := New()
	err := c.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if c.URL != "https://file.lab/sdk" {
		t.Errorf("URL = %q, want %q (file should override default)", c.URL, "https://file.lab/sdk")
	}
	if c.Username != "fileuser" {
		t.Errorf("Username = %q, want %q (file should override default)", c.Username, "fileuser")
	}
	if c.Password != "filepass" {
		t.Errorf("Password = %q, want %q (file should override default)", c.Password, "filepass")
	}
	if c.Insecure {
		t.Error("Insecure should be false (file should override default)")
	}
	if c.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s (file should override default)", c.Timeout)
	}
}

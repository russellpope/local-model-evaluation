package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func setupTestViper(t *testing.T, configContent string, envVars map[string]string, flagValues map[string]string) *viper.Viper {
	t.Helper()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	if configContent != "" {
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("write config file: %v", err)
		}
	}

	for k, v := range envVars {
		t.Setenv(k, v)
	}

	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("timeout", DefaultTimeout)
	v.SetDefault("insecure", false)

	if configContent != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			t.Fatalf("read config file: %v", err)
		}
	}

	flagSet := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flagSet.String("url", "", "vCenter URL")
	flagSet.String("username", "", "vCenter username")
	flagSet.String("password", "", "vCenter password")
	flagSet.Bool("insecure", false, "skip TLS verification")
	flagSet.String("timeout", "60s", "operation timeout")

	for name, value := range flagValues {
		if err := flagSet.Set(name, value); err != nil {
			t.Fatalf("set flag %s: %v", name, err)
		}
	}

	for _, name := range []string{"url", "username", "password", "insecure", "timeout"} {
		v.BindPFlag(name, flagSet.Lookup(name))
	}

	return v
}

func TestConfigPrecedence(t *testing.T) {
	configContent := `
url: https://config-file.example.com/sdk
username: configuser
password: configpass
insecure: false
timeout: 30s
`
	envVars := map[string]string{
		"VSPHERE_URL":      "https://env-var.example.com/sdk",
		"VSPHERE_USERNAME": "envuser",
		"VSPHERE_PASSWORD": "envpass",
	}
	flagValues := map[string]string{
		"url":      "https://flag.example.com/sdk",
		"username": "flaguser",
		"password": "flagpass",
	}

	v := setupTestViper(t, configContent, envVars, flagValues)

	cfg, err := FromViper(v)
	if err != nil {
		t.Fatalf("FromViper: %v", err)
	}

	if cfg.URL != "https://flag.example.com/sdk" {
		t.Errorf("URL: got %q, want %q (flag should override env and config)", cfg.URL, "https://flag.example.com/sdk")
	}
	if cfg.Username != "flaguser" {
		t.Errorf("Username: got %q, want %q (flag should override env and config)", cfg.Username, "flaguser")
	}
	if cfg.Password != "flagpass" {
		t.Errorf("Password: got %q, want %q (flag should override env and config)", cfg.Password, "flagpass")
	}
}

func TestConfigPrecedenceEnvOverFile(t *testing.T) {
	configContent := `
url: https://config-file.example.com/sdk
username: configuser
password: configpass
timeout: 30s
`
	envVars := map[string]string{
		"VSPHERE_URL":      "https://env-var.example.com/sdk",
		"VSPHERE_USERNAME": "envuser",
	}

	v := setupTestViper(t, configContent, envVars, nil)

	cfg, err := FromViper(v)
	if err != nil {
		t.Fatalf("FromViper: %v", err)
	}

	if cfg.URL != "https://env-var.example.com/sdk" {
		t.Errorf("URL: got %q, want %q (env should override config file)", cfg.URL, "https://env-var.example.com/sdk")
	}
	if cfg.Username != "envuser" {
		t.Errorf("Username: got %q, want %q (env should override config file)", cfg.Username, "envuser")
	}
	if cfg.Password != "configpass" {
		t.Errorf("Password: got %q, want %q (should come from config file)", cfg.Password, "configpass")
	}
}

func TestConfigPrecedenceFileOverDefault(t *testing.T) {
	configContent := `
url: https://config-file.example.com/sdk
username: configuser
password: configpass
timeout: 30s
`

	v := setupTestViper(t, configContent, nil, nil)

	cfg, err := FromViper(v)
	if err != nil {
		t.Fatalf("FromViper: %v", err)
	}

	if cfg.URL != "https://config-file.example.com/sdk" {
		t.Errorf("URL: got %q, want %q (config file should override default)", cfg.URL, "https://config-file.example.com/sdk")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout: got %v, want %v (config file should override default)", cfg.Timeout, 30*time.Second)
	}
}

func TestConfigDefaultTimeout(t *testing.T) {
	v := setupTestViper(t, "", nil, nil)

	cfg, err := FromViper(v)
	if err != nil {
		t.Fatalf("FromViper: %v", err)
	}

	expected, _ := time.ParseDuration(DefaultTimeout)
	if cfg.Timeout != expected {
		t.Errorf("Timeout: got %v, want %v (default)", cfg.Timeout, expected)
	}
}

func TestConfigDefaultInsecure(t *testing.T) {
	v := setupTestViper(t, "", nil, nil)

	cfg, err := FromViper(v)
	if err != nil {
		t.Fatalf("FromViper: %v", err)
	}

	if cfg.Insecure != false {
		t.Errorf("Insecure: got %v, want false (default)", cfg.Insecure)
	}
}

func TestConfigFlagInsecure(t *testing.T) {
	v := setupTestViper(t, "", nil, map[string]string{
		"insecure": "true",
	})

	cfg, err := FromViper(v)
	if err != nil {
		t.Fatalf("FromViper: %v", err)
	}

	if cfg.Insecure != true {
		t.Errorf("Insecure: got %v, want true (flag)", cfg.Insecure)
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     &Config{URL: "https://vc.example.com/sdk", Username: "user", Password: "pass"},
			wantErr: false,
		},
		{
			name:    "missing URL",
			cfg:     &Config{Username: "user", Password: "pass"},
			wantErr: true,
		},
		{
			name:    "missing username",
			cfg:     &Config{URL: "https://vc.example.com/sdk", Password: "pass"},
			wantErr: true,
		},
		{
			name:    "missing password",
			cfg:     &Config{URL: "https://vc.example.com/sdk", Username: "user"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

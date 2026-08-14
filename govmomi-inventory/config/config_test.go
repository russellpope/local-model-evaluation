package config

import (
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestConfigPrecedence(t *testing.T) {
	cfgContent := "url: https://file.example.com/sdk\nusername: fileuser\npassword: filepass\n"
	cfgPath, err := WriteTestConfig(cfgContent)
	if err != nil {
		t.Fatalf("write test config: %v", err)
	}
	defer os.Remove(cfgPath)

	v := NewViper()
	if err := ReadConfigFile(v, cfgPath); err != nil {
		t.Fatalf("read config file: %v", err)
	}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := BindFlags(v, flags); err != nil {
		t.Fatalf("bind flags: %v", err)
	}

	os.Setenv("VSPHERE_URL", "https://env.example.com/sdk")
	os.Setenv("VSPHERE_USERNAME", "envuser")
	os.Setenv("VSPHERE_PASSWORD", "envpass")
	defer func() {
		os.Unsetenv("VSPHERE_URL")
		os.Unsetenv("VSPHERE_USERNAME")
		os.Unsetenv("VSPHERE_PASSWORD")
	}()

	if err := flags.Parse([]string{"--url", "https://flag.example.com/sdk", "--username", "flaguser", "--password", "flagpass"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	urlVal, _ := flags.GetString("url")
	usernameVal, _ := flags.GetString("username")
	passwordVal, _ := flags.GetString("password")
	v.Set("url", urlVal)
	v.Set("username", usernameVal)
	v.Set("password", passwordVal)

	cfg := LoadConfig(v)

	if cfg.URL != "https://flag.example.com/sdk" {
		t.Errorf("URL precedence: got %q, want flag value", cfg.URL)
	}
	if cfg.Username != "flaguser" {
		t.Errorf("Username precedence: got %q, want flag value", cfg.Username)
	}
	if cfg.Password != "flagpass" {
		t.Errorf("Password precedence: got %q, want flag value", cfg.Password)
	}
}

func TestConfigEnvOverridesFile(t *testing.T) {
	cfgContent := "url: https://file.example.com/sdk\nusername: fileuser\n"
	cfgPath, err := WriteTestConfig(cfgContent)
	if err != nil {
		t.Fatalf("write test config: %v", err)
	}
	defer os.Remove(cfgPath)

	os.Setenv("VSPHERE_URL", "https://env.example.com/sdk")
	os.Setenv("VSPHERE_USERNAME", "envuser")
	defer func() {
		os.Unsetenv("VSPHERE_URL")
		os.Unsetenv("VSPHERE_USERNAME")
	}()

	v := NewViper()
	if err := ReadConfigFile(v, cfgPath); err != nil {
		t.Fatalf("read config file: %v", err)
	}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := BindFlags(v, flags); err != nil {
		t.Fatalf("bind flags: %v", err)
	}
	if err := flags.Parse([]string{}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	cfg := LoadConfig(v)

	if cfg.URL != "https://env.example.com/sdk" {
		t.Errorf("URL env override: got %q, want env value", cfg.URL)
	}
	if cfg.Username != "envuser" {
		t.Errorf("Username env override: got %q, want env value", cfg.Username)
	}
}

func TestConfigDefaults(t *testing.T) {
	v := NewViper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := BindFlags(v, flags); err != nil {
		t.Fatalf("bind flags: %v", err)
	}
	if err := flags.Parse([]string{}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	cfg := LoadConfig(v)
	if cfg.Timeout != 60*time.Second {
		t.Errorf("default timeout: got %v, want 60s", cfg.Timeout)
	}
	if cfg.Insecure != false {
		t.Errorf("default insecure: got %v, want false", cfg.Insecure)
	}
}

func TestConfigFileOverridesDefault(t *testing.T) {
	cfgContent := "url: https://file.example.com/sdk\n"
	cfgPath, err := WriteTestConfig(cfgContent)
	if err != nil {
		t.Fatalf("write test config: %v", err)
	}
	defer os.Remove(cfgPath)

	v := NewViper()
	if err := ReadConfigFile(v, cfgPath); err != nil {
		t.Fatalf("read config file: %v", err)
	}

	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	if err := BindFlags(v, flags); err != nil {
		t.Fatalf("bind flags: %v", err)
	}
	if err := flags.Parse([]string{}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	cfg := LoadConfig(v)
	if cfg.URL != "https://file.example.com/sdk" {
		t.Errorf("file override: got %q, want file value", cfg.URL)
	}
}

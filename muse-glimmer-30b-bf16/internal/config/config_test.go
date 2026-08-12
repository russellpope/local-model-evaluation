package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestConfigPrecedence(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")
	os.WriteFile(cfgPath, []byte("url: file-url\nusername: file-user\npassword: file-pass\n"), 0644)
	viper.Reset()
	Init(cfgPath)
	os.Setenv("VSPHERE_URL", "env-url")
	os.Setenv("VSPHERE_USERNAME", "env-user")
	os.Setenv("VSPHERE_PASSWORD", "env-pass")
	defer os.Unsetenv("VSPHERE_URL")
	defer os.Unsetenv("VSPHERE_USERNAME")
	defer os.Unsetenv("VSPHERE_PASSWORD")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.URL != "env-url" {
		t.Errorf("env not precedence over file: %s", cfg.URL)
	}
	viper.Set("url", "flag-url")
	cfg2, _ := Load()
	if cfg2.URL != "flag-url" {
		t.Errorf("flag not precedence")
	}
}

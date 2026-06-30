package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestResolvePrecedence(t *testing.T) {
	v := viper.New()
	
	// Default
	v.SetDefault("url", "http://default")
	v.SetDefault("timeout", "60s")
	
	// Config file simulation
	v.Set("url", "http://file")
	
	// Env var
	os.Setenv("VSPHERE_URL", "http://env")
	defer os.Unsetenv("VSPHERE_URL")
	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()
	
	// Flag (manually set in viper)
	v.Set("url", "http://flag")
	
	cfg, err := Resolve(v)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	
	if cfg.URL != "http://flag" {
		t.Errorf("expected flag to override, got %s", cfg.URL)
	}
}

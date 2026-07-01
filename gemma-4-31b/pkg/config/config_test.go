package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestResolvePrecedence(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*viper.Viper)
		expected string
	}{
		{
			name: "Default",
			setup: func(v *viper.Viper) {
				v.SetDefault("url", "http://default")
				v.SetDefault("timeout", "60s")
			},
			expected: "http://default",
		},
		{
			name: "ConfigFileOverridesDefault",
			setup: func(v *viper.Viper) {
				v.SetDefault("url", "http://default")
				v.SetDefault("timeout", "60s")
				v.Set("url", "http://file")
			},
			expected: "http://file",
		},
		{
			name: "EnvVarOverridesConfigFile",
			setup: func(v *viper.Viper) {
				v.SetDefault("url", "http://default")
				v.SetDefault("timeout", "60s")
				v.Set("url", "http://file")
				os.Setenv("VSPHERE_URL", "http://env")
				v.SetEnvPrefix("VSPHERE")
				v.AutomaticEnv()
			},
			expected: "http://env",
		},
		{
			name: "FlagOverridesEnvVar",
			setup: func(v *viper.Viper) {
				v.SetDefault("url", "http://default")
				v.SetDefault("timeout", "60s")
				v.Set("url", "http://file")
				os.Setenv("VSPHERE_URL", "http://env")
				v.SetEnvPrefix("VSPHERE")
				v.AutomaticEnv()
				v.Set("url", "http://flag")
			},
			expected: "http://flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			v := viper.New()
			tt.setup(v)
			cfg, err := Resolve(v)
			if err != nil {
				t.Fatalf("Resolve failed: %v", err)
			}
			if cfg.URL != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, cfg.URL)
			}
		})
	}
}

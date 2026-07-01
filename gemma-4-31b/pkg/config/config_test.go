package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
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

				tmpFile := filepath.Join(t.TempDir(), "config.yaml")
				os.WriteFile(tmpFile, []byte("url: http://file"), 0644)
				v.SetConfigFile(tmpFile)
				if err := v.ReadInConfig(); err != nil {
					t.Fatalf("failed to read config: %v", err)
				}
			},
			expected: "http://file",
		},
		{
			name: "EnvVarOverridesConfigFile",
			setup: func(v *viper.Viper) {
				v.SetDefault("url", "http://default")
				v.SetDefault("timeout", "60s")

				tmpFile := filepath.Join(t.TempDir(), "config.yaml")
				os.WriteFile(tmpFile, []byte("url: http://file"), 0644)
				v.SetConfigFile(tmpFile)
				v.ReadInConfig()

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

				tmpFile := filepath.Join(t.TempDir(), "config.yaml")
				os.WriteFile(tmpFile, []byte("url: http://file"), 0644)
				v.SetConfigFile(tmpFile)
				v.ReadInConfig()

				os.Setenv("VSPHERE_URL", "http://env")
				v.SetEnvPrefix("VSPHERE")
				v.AutomaticEnv()

				fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
				fs.String("url", "", "URL")
				fs.Parse([]string{"--url", "http://flag"})
				v.BindPFlag("url", fs.Lookup("url"))
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

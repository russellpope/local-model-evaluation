// Package config resolves the shared connection configuration.
// Precedence (highest first): command-line flag, environment variable
// (VSPHERE_*), YAML config file, built-in default.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the resolved shared connection configuration.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// NewViper wires the VSPHERE_ environment prefix and the built-in defaults
// for the precedence chain. Flag bindings are added by the cmd package via
// BindPFlags so this package stays free of CLI plumbing.
func NewViper() *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix("VSPHERE")
	v.AutomaticEnv()
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", "60s")
	return v
}

// Resolve merges the precedence layers into a Config. The config key (set by
// --config or VSPHERE_CONFIG) selects the YAML file layer, if given.
func Resolve(v *viper.Viper) (Config, error) {
	if path := v.GetString("config"); path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return Config{}, fmt.Errorf("read config file %q: %w", path, err)
		}
	}
	cfg := Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
	}
	if cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("invalid timeout %q (set --timeout, VSPHERE_TIMEOUT, or timeout in the config file)", v.Get("timeout"))
	}
	return cfg, nil
}

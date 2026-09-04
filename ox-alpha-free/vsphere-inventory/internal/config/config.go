// Package config resolves vCenter connection settings through Viper with the
// precedence: command-line flag > environment variable > config file > default.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// EnvPrefix is prepended to every configuration key when reading
// environment variables (e.g. key "url" reads VSPHERE_URL).
const EnvPrefix = "VSPHERE"

// Config holds the resolved connection settings shared by all subcommands.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// FlagValues carries raw command-line flag values keyed by config key.
// A key's presence means the flag was explicitly set on the command line,
// which puts it above environment variables and the config file in precedence.
type FlagValues map[string]string

var keys = []string{"url", "username", "password", "insecure", "timeout"}

// Load resolves the effective configuration. flags contains only explicitly
// set command-line flags; configFile may be empty.
func Load(flags FlagValues, configFile string) (*Config, error) {
	v := viper.New()

	v.SetEnvPrefix(EnvPrefix)
	for _, k := range keys {
		if err := v.BindEnv(k); err != nil {
			return nil, fmt.Errorf("bind env var for %q: %w", k, err)
		}
	}

	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", "60s")

	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file %q: %w", configFile, err)
		}
	}

	for k, val := range flags {
		v.Set(k, val)
	}

	cfg := &Config{
		URL:      v.GetString("url"),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("no vCenter URL configured; use --url, set %s_URL, or add 'url' to a config file passed via --config", EnvPrefix)
	}
	if cfg.Timeout <= 0 {
		return nil, fmt.Errorf("invalid timeout %s: must be greater than zero (e.g. --timeout 60s)", cfg.Timeout)
	}
	return cfg, nil
}

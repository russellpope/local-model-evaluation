package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix is the environment variable prefix for all configuration keys.
const EnvPrefix = "VSPHERE"

// Keys are the configuration keys shared by every subcommand.
var Keys = []string{"url", "username", "password", "insecure", "timeout"}

// Built-in defaults.
const (
	DefaultInsecure = false
	DefaultTimeout  = 60 * time.Second
)

// Config holds the resolved connection settings for one subcommand run.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// NewViper builds a Viper instance with the layering (lowest to highest
// precedence): built-in defaults < config file < environment variables
// (VSPHERE_*) < command-line flags (bound via BindFlags).
//
// configFile may be empty, in which case no file layer is used.
func NewViper(configFile string) (*viper.Viper, error) {
	v := viper.New()

	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", DefaultInsecure)
	v.SetDefault("timeout", DefaultTimeout.String())

	if configFile != "" {
		v.SetConfigFile(configFile)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file %q: %w", configFile, err)
		}
	}

	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	return v, nil
}

// BindFlags binds the standard connection flags to v so that a flag the user
// set on the command line overrides environment variables and the config file.
// Flags left at their default do not override lower layers.
func BindFlags(v *viper.Viper, fs *pflag.FlagSet) error {
	for _, key := range Keys {
		f := fs.Lookup(key)
		if f == nil {
			return fmt.Errorf("flag --%s is not defined on the command", key)
		}
		if err := v.BindPFlag(key, f); err != nil {
			return fmt.Errorf("bind flag --%s: %w", key, err)
		}
	}
	return nil
}

// Load resolves the final configuration from a fully-wired Viper instance.
func Load(v *viper.Viper) (Config, error) {
	cfg := Config{
		URL:      strings.TrimSpace(v.GetString("url")),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
	}

	raw := v.GetString("timeout")
	d, err := time.ParseDuration(raw)
	if err != nil {
		return Config{}, fmt.Errorf("invalid timeout %q: %w", raw, err)
	}
	if d <= 0 {
		return Config{}, fmt.Errorf("timeout must be a positive duration, got %q", raw)
	}
	cfg.Timeout = d

	if cfg.URL == "" {
		return Config{}, fmt.Errorf("no vCenter URL configured; set --url, the %s_URL environment variable, or url in the config file", EnvPrefix)
	}

	return cfg, nil
}

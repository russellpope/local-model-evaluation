// Package config resolves the shared vCenter connection settings with the
// precedence: command-line flag > environment variable > config file > default.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds the connection settings shared by all subcommands.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// Keys are the configuration keys shared by flags (--key), environment
// variables (VSPHERE_<KEY>) and the YAML config file.
var Keys = []string{"url", "username", "password", "insecure", "timeout"}

const envPrefix = "VSPHERE"

// NewViper builds a configured viper instance. Precedence is handled by
// viper itself once the flags are bound via BindFlags: an explicitly set
// flag beats an environment variable, which beats the config file, which
// beats the built-in default.
func NewViper(configFile string) (*viper.Viper, error) {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()

	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", "60s")

	if configFile != "" {
		v.SetConfigFile(configFile)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file %s: %w", configFile, err)
		}
	}

	return v, nil
}

// BindFlags binds the persistent connection flags to viper so that changed
// flags take the highest precedence.
func BindFlags(v *viper.Viper, fs *pflag.FlagSet) error {
	for _, key := range Keys {
		f := fs.Lookup(key)
		if f == nil {
			return fmt.Errorf("flag --%s is not defined", key)
		}
		if err := v.BindPFlag(key, f); err != nil {
			return fmt.Errorf("bind flag --%s: %w", key, err)
		}
	}
	return nil
}

// Load resolves the final Config values from a configured viper instance.
func Load(v *viper.Viper) (Config, error) {
	cfg := Config{
		URL:      strings.TrimSpace(v.GetString("url")),
		Username: v.GetString("username"),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
	}

	timeout, err := parseTimeout(v.Get("timeout"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid timeout value: %w", err)
	}
	cfg.Timeout = timeout

	return cfg, nil
}

func parseTimeout(v any) (time.Duration, error) {
	var d time.Duration

	switch x := v.(type) {
	case time.Duration:
		d = x
	case string:
		if strings.TrimSpace(x) == "" {
			return 0, fmt.Errorf("timeout is empty")
		}
		parsed, err := time.ParseDuration(x)
		if err != nil {
			return 0, fmt.Errorf("parse %q: %w", x, err)
		}
		d = parsed
	case int:
		d = time.Duration(x) * time.Second
	case int64:
		d = time.Duration(x) * time.Second
	default:
		return 0, fmt.Errorf("unsupported timeout type %T", v)
	}

	if d <= 0 {
		return 0, fmt.Errorf("timeout must be positive, got %s", d)
	}

	return d, nil
}

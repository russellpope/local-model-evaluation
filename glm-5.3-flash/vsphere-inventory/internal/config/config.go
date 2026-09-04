// Package config resolves the shared vCenter connection settings.
//
// Precedence (highest first): command-line flag > environment variable >
// YAML config file > built-in default. The resolution itself is delegated to
// viper: defaults via SetDefault, the file via ReadInConfig, environment
// variables via BindEnv with the VSPHERE_ prefix, and flags via BindPFlags
// (viper only consults a bound flag when the user actually set it).
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// EnvPrefix is prepended to every environment variable name.
const EnvPrefix = "VSPHERE"

// DefaultTimeout is used when no other source provides a timeout.
const DefaultTimeout = 60 * time.Second

// Config holds the connection settings shared by all subcommands.
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// AddFlags registers the connection flags on fs. They are bound into viper
// by Load. The --config flag names the optional YAML config file.
func AddFlags(fs *pflag.FlagSet) {
	fs.String("url", "", "vCenter URL or host, e.g. https://vc.lab/sdk")
	fs.String("username", "", "vCenter username")
	fs.String("password", "", "vCenter password")
	fs.Bool("insecure", false, "skip TLS certificate verification")
	fs.Duration("timeout", DefaultTimeout, "overall operation timeout, e.g. 60s")
	fs.String("config", "", "path to a YAML config file")
}

// Load resolves the configuration from the given flag set (which must have
// been populated by AddFlags), the VSPHERE_* environment variables, and the
// optional YAML file named by --config.
func Load(fs *pflag.FlagSet) (*Config, error) {
	v := viper.New()

	v.SetDefault("url", "")
	v.SetDefault("username", "")
	v.SetDefault("password", "")
	v.SetDefault("insecure", false)
	v.SetDefault("timeout", DefaultTimeout.String())

	v.SetEnvPrefix(EnvPrefix)
	for _, key := range []string{"url", "username", "password", "insecure", "timeout"} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind env for %q: %w", key, err)
		}
	}
	if err := v.BindPFlags(fs); err != nil {
		return nil, fmt.Errorf("bind flags: %w", err)
	}

	if path := strings.TrimSpace(v.GetString("config")); path != "" {
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("read config file %q: %w", path, err)
		}
	}

	cfg := &Config{
		URL:      strings.TrimSpace(v.GetString("url")),
		Username: strings.TrimSpace(v.GetString("username")),
		Password: v.GetString("password"),
		Insecure: v.GetBool("insecure"),
		Timeout:  v.GetDuration("timeout"),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate rejects configurations that could never connect, with messages
// that name every way the missing value can be supplied.
func (c *Config) Validate() error {
	if c.URL == "" {
		return errors.New(`no vCenter URL configured: set --url, $VSPHERE_URL, or "url" in the config file`)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("invalid timeout %s: must be positive, e.g. 60s", c.Timeout)
	}
	return nil
}
